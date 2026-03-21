package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	appstorage "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/tracker"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/config"
	grpccontroller "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/controller/grpc"
	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/grpc"
	ghclient "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/github"
	stackclient "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/stackoverflow"
	migrateinfra "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/migrate"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/sql"
	squirrelrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/squirrel"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/pkg/grpcx"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

//nolint:funlen // wiring/DI entrypoint intentionally composes many infra components.
func Run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.DBAutoMigrate {
		if migrationErr := migrateinfra.Up(cfg.DBMigrationsPath, cfg.DBDsn); migrationErr != nil {
			return fmt.Errorf("run migrations: %w", migrationErr)
		}
	}

	pgxCfg, err := pgxpool.ParseConfig(cfg.DBDsn)
	if err != nil {
		return fmt.Errorf("parse DB_DSN: %w", err)
	}
	pgxCfg.MaxConns = cfg.DBMaxConns
	pgxCfg.MinConns = cfg.DBMinConns

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		return fmt.Errorf("create pgx pool: %w", err)
	}
	defer pool.Close()

	pingErr := pool.Ping(context.Background())
	if pingErr != nil {
		return fmt.Errorf("ping postgres: %w", pingErr)
	}

	repo, err := newRepository(cfg.DBAccessType, pool)
	if err != nil {
		return err
	}

	botClient, err := grpcadapter.NewBotClient(cfg.BotGRPCAddr, cfg.GRPCTimeout, logger)
	if err != nil {
		return fmt.Errorf("create bot grpc client: %w", err)
	}
	defer func() {
		if closeErr := botClient.Close(); closeErr != nil {
			logger.Warn("failed to close bot grpc client", slog.String("error", closeErr.Error()))
		}
	}()

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	githubClient := ghclient.New(cfg.GitHubBaseURL, httpClient)
	stackOverflowClient := stackclient.New(cfg.StackBaseURL, httpClient)

	trackerService := tracker.New(repo, githubClient, stackOverflowClient, botClient, logger)
	grpcSvc := grpccontroller.NewServer(repo, trackerService)

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", cfg.ScrapperGRPCAddr)
	if err != nil {
		return fmt.Errorf("listen scrapper grpc on %s: %w", cfg.ScrapperGRPCAddr, err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerLogger(logger)))
	pb.RegisterScrapperServiceServer(grpcServer, grpcSvc)

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("scrapper grpc server started", slog.String("addr", cfg.ScrapperGRPCAddr))
		if serveErrValue := grpcServer.Serve(listener); serveErrValue != nil {
			if !errors.Is(serveErrValue, grpc.ErrServerStopped) {
				serveErr <- serveErrValue
			}
		}
	}()

	scheduler := gocron.NewScheduler(time.UTC)
	_, err = scheduler.Every(cfg.SchedulerInterval).Do(func() {
		trackerService.CheckUpdates(context.Background())
	})
	if err != nil {
		return fmt.Errorf("schedule tracking job: %w", err)
	}
	scheduler.StartAsync()
	logger.Info("scrapper scheduler started", slog.String("interval", cfg.SchedulerInterval.String()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("shutting down scrapper")
		scheduler.Stop()
		grpcServer.GracefulStop()
		return nil
	case grpcServeErr := <-serveErr:
		scheduler.Stop()
		grpcServer.GracefulStop()
		return fmt.Errorf("scrapper grpc server stopped unexpectedly: %w", grpcServeErr)
	}
}

func newRepository(accessType string, pool *pgxpool.Pool) (appstorage.Repository, error) {
	switch accessType {
	case "sql":
		return sqlrepo.New(pool), nil
	case "squirrel":
		return squirrelrepo.New(pool), nil
	default:
		return nil, fmt.Errorf("unsupported DB_ACCESS_TYPE: %s", accessType)
	}
}
