package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/handler"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/memory"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	bot, err := telegram.New(cfg.AppTelegramToken)
	if err != nil {
		logger.Error("failed to create bot", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("bot authorized", slog.String("username", bot.GetUserName()))

	userRepo := memory.NewUserRepository()
	userUseCase := usecase.NewUserUseCase(userRepo)
	h := handler.New(bot, userUseCase, logger)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("bot started, listening for updates...")

	for {
		select {
		case update := <-updates:
			h.HandleUpdate(update)
		case sig := <-quit:
			logger.Info("shutting down bot", slog.String("signal", sig.String()))
			bot.StopReceivingUpdates()

			return
		}
	}
}
