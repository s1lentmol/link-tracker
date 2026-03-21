package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	configpkg "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/pkg/config"
)

type Config struct {
	ScrapperGRPCAddr  string        `mapstructure:"scrapper_grpc_addr"`
	BotGRPCAddr       string        `mapstructure:"bot_grpc_addr"`
	DBDsn             string        `mapstructure:"db_dsn"`
	DBAccessType      string        `mapstructure:"db_access_type"`
	DBMigrationsPath  string        `mapstructure:"db_migrations_path"`
	DBAutoMigrate     bool          `mapstructure:"db_auto_migrate"`
	DBMaxConns        int32         `mapstructure:"db_max_conns"`
	DBMinConns        int32         `mapstructure:"db_min_conns"`
	SchedulerInterval time.Duration `mapstructure:"scheduler_interval"`
	HTTPTimeout       time.Duration `mapstructure:"http_timeout"`
	GRPCTimeout       time.Duration `mapstructure:"grpc_timeout"`
	GitHubBaseURL     string        `mapstructure:"github_base_url"`
	StackBaseURL      string        `mapstructure:"stack_base_url"`
}

const defaultDBMaxConns = 10

//nolint:funlen // config loading validates many independent environment fields.
func Load() (*Config, error) {
	v, err := configpkg.NewViperFromEnvFile(".env")
	if err != nil {
		return nil, fmt.Errorf("new viper: %w", err)
	}

	_ = v.BindEnv("scrapper_grpc_addr")
	_ = v.BindEnv("bot_grpc_addr")
	_ = v.BindEnv("db_dsn")
	_ = v.BindEnv("db_access_type")
	_ = v.BindEnv("db_migrations_path")
	_ = v.BindEnv("db_auto_migrate")
	_ = v.BindEnv("db_max_conns")
	_ = v.BindEnv("db_min_conns")
	_ = v.BindEnv("scheduler_interval")
	_ = v.BindEnv("http_timeout")
	_ = v.BindEnv("grpc_timeout")
	_ = v.BindEnv("github_base_url")
	_ = v.BindEnv("stack_base_url")

	v.SetDefault("scrapper_grpc_addr", ":8081")
	v.SetDefault("bot_grpc_addr", "localhost:8082")
	v.SetDefault("db_dsn", "postgres://postgres:postgres@localhost:5432/linktracker?sslmode=disable")
	v.SetDefault("db_access_type", "sql")
	v.SetDefault("db_migrations_path", "migrations")
	v.SetDefault("db_auto_migrate", true)
	v.SetDefault("db_max_conns", defaultDBMaxConns)
	v.SetDefault("db_min_conns", 1)
	v.SetDefault("scheduler_interval", "30s")
	v.SetDefault("http_timeout", "10s")
	v.SetDefault("grpc_timeout", "3s")
	v.SetDefault("github_base_url", "https://api.github.com")
	v.SetDefault("stack_base_url", "https://api.stackexchange.com/2.3")

	var cfg Config
	unmarshalErr := v.Unmarshal(&cfg)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", unmarshalErr)
	}

	cfg.ScrapperGRPCAddr = strings.TrimSpace(cfg.ScrapperGRPCAddr)
	cfg.BotGRPCAddr = strings.TrimSpace(cfg.BotGRPCAddr)
	cfg.DBDsn = strings.TrimSpace(cfg.DBDsn)
	cfg.DBAccessType = strings.TrimSpace(strings.ToLower(cfg.DBAccessType))
	cfg.DBMigrationsPath = strings.TrimSpace(cfg.DBMigrationsPath)
	cfg.GitHubBaseURL = strings.TrimSpace(cfg.GitHubBaseURL)
	cfg.StackBaseURL = strings.TrimSpace(cfg.StackBaseURL)

	if cfg.ScrapperGRPCAddr == "" {
		return nil, errors.New("SCRAPPER_GRPC_ADDR is required")
	}
	if cfg.BotGRPCAddr == "" {
		return nil, errors.New("BOT_GRPC_ADDR is required")
	}
	if cfg.DBDsn == "" {
		return nil, errors.New("DB_DSN is required")
	}
	if cfg.DBAccessType != "sql" && cfg.DBAccessType != "squirrel" {
		return nil, errors.New("DB_ACCESS_TYPE must be one of: sql, squirrel")
	}
	if cfg.DBMigrationsPath == "" {
		return nil, errors.New("DB_MIGRATIONS_PATH is required")
	}
	if cfg.DBMaxConns <= 0 {
		return nil, errors.New("DB_MAX_CONNS must be positive")
	}
	if cfg.DBMinConns < 0 {
		return nil, errors.New("DB_MIN_CONNS must be non-negative")
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		return nil, errors.New("DB_MIN_CONNS must be <= DB_MAX_CONNS")
	}
	if cfg.SchedulerInterval <= 0 {
		return nil, errors.New("SCHEDULER_INTERVAL must be positive")
	}
	if cfg.HTTPTimeout <= 0 {
		return nil, errors.New("HTTP_TIMEOUT must be positive")
	}
	if cfg.GRPCTimeout <= 0 {
		return nil, errors.New("GRPC_TIMEOUT must be positive")
	}
	if cfg.GitHubBaseURL == "" || cfg.StackBaseURL == "" {
		return nil, errors.New("GITHUB_BASE_URL and STACK_BASE_URL are required")
	}

	return &cfg, nil
}
