package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/grpc"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/user"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	handler "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/controller/telegram"
	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/grpcx"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/shared/pb"
)

func Run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	bot, err := telegram.New(cfg.AppTelegramToken)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	scrapperClient, err := grpcadapter.NewScrapperClient(cfg.ScrapperGRPCAddr, cfg.GRPCTimeout, logger)
	if err != nil {
		return fmt.Errorf("create scrapper grpc client: %w", err)
	}
	defer func() {
		if err := scrapperClient.Close(); err != nil {
			logger.Warn("failed to close scrapper grpc connection", slog.String("error", err.Error()))
		}
	}()

	grpcListener, err := net.Listen("tcp", cfg.BotGRPCAddr)
	if err != nil {
		return fmt.Errorf("listen bot grpc on %s: %w", cfg.BotGRPCAddr, err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerLogger(logger)))
	pb.RegisterBotServiceServer(grpcServer, grpcadapter.NewBotUpdatesServer(bot, logger))

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("bot grpc server started", slog.String("addr", cfg.BotGRPCAddr))
		if err := grpcServer.Serve(grpcListener); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				serveErr <- err
			}
		}
	}()

	logger.Info("bot authorized", slog.String("username", bot.GetUserName()))

	userRepo := storage.NewUserRepository()
	userUseCase := user.NewUseCase(userRepo)
	h := handler.New(bot, userUseCase, logger, handler.WithScrapperService(scrapperClient))

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
				grpcServer.GracefulStop()
				return nil
			}
			h.HandleUpdate(update)
		case sig := <-quit:
			logger.Info("shutting down bot", slog.String("signal", sig.String()))
			bot.StopReceivingUpdates()
			grpcServer.GracefulStop()
			return nil
		case err := <-serveErr:
			bot.StopReceivingUpdates()
			grpcServer.GracefulStop()
			return fmt.Errorf("bot grpc server stopped unexpectedly: %w", err)
		}
	}
}
