package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	configpkg "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/pkg/config"
)

type Config struct {
	AppTelegramToken string        `mapstructure:"app_telegram_token"`
	BotGRPCAddr      string        `mapstructure:"bot_grpc_addr"`
	ScrapperGRPCAddr string        `mapstructure:"scrapper_grpc_addr"`
	GRPCTimeout      time.Duration `mapstructure:"grpc_timeout"`
}

func Load() (*Config, error) {
	v, err := configpkg.NewViperFromEnvFile(".env")
	if err != nil {
		return nil, fmt.Errorf("new viper: %w", err)
	}

	_ = v.BindEnv("app_telegram_token")
	_ = v.BindEnv("bot_grpc_addr")
	_ = v.BindEnv("scrapper_grpc_addr")
	_ = v.BindEnv("grpc_timeout")

	v.SetDefault("bot_grpc_addr", ":8082")
	v.SetDefault("scrapper_grpc_addr", "localhost:8081")
	v.SetDefault("grpc_timeout", "3s")

	var cfg Config
	unmarshalErr := v.Unmarshal(&cfg)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", unmarshalErr)
	}

	cfg.AppTelegramToken = strings.TrimSpace(cfg.AppTelegramToken)
	cfg.BotGRPCAddr = strings.TrimSpace(cfg.BotGRPCAddr)
	cfg.ScrapperGRPCAddr = strings.TrimSpace(cfg.ScrapperGRPCAddr)

	if cfg.AppTelegramToken == "" {
		return nil, errors.New("APP_TELEGRAM_TOKEN is required")
	}
	if cfg.BotGRPCAddr == "" {
		return nil, errors.New("BOT_GRPC_ADDR is required")
	}
	if cfg.ScrapperGRPCAddr == "" {
		return nil, errors.New("SCRAPPER_GRPC_ADDR is required")
	}
	if cfg.GRPCTimeout <= 0 {
		return nil, errors.New("GRPC_TIMEOUT must be positive")
	}

	return &cfg, nil
}
