package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/internal/adapter/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/internal/adapter/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/config"
	handler "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/internal/controller/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/internal/usecase/user"
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

	userRepo := storage.NewUserRepository()
	userUseCase := user.NewUseCase(userRepo)
	h := handler.New(bot, userUseCase, logger)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("bot started, listening for updates...")

	for {
		select {
		case update, ok := <-updates:
			if !ok {
				logger.Warn("updates channel closed, shutting down bot loop")
				return
			}

			h.HandleUpdate(update)
		case sig := <-quit:
			logger.Info("shutting down bot", slog.String("signal", sig.String()))
			bot.StopReceivingUpdates()

			return
		}
	}
}
