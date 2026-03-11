package config

import (
	"fmt"
	"strings"
	"time"

	configpkg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

type Config struct {
	ScrapperGRPCAddr  string        `mapstructure:"scrapper_grpc_addr"`
	BotGRPCAddr       string        `mapstructure:"bot_grpc_addr"`
	SchedulerInterval time.Duration `mapstructure:"scheduler_interval"`
	HTTPTimeout       time.Duration `mapstructure:"http_timeout"`
	GRPCTimeout       time.Duration `mapstructure:"grpc_timeout"`
	GitHubBaseURL     string        `mapstructure:"github_base_url"`
	StackBaseURL      string        `mapstructure:"stack_base_url"`
}

func Load() (*Config, error) {
	v, err := configpkg.NewViperFromEnvFile(".env")
	if err != nil {
		return nil, err
	}

	_ = v.BindEnv("scrapper_grpc_addr")
	_ = v.BindEnv("bot_grpc_addr")
	_ = v.BindEnv("scheduler_interval")
	_ = v.BindEnv("http_timeout")
	_ = v.BindEnv("grpc_timeout")
	_ = v.BindEnv("github_base_url")
	_ = v.BindEnv("stack_base_url")

	v.SetDefault("scrapper_grpc_addr", ":8081")
	v.SetDefault("bot_grpc_addr", "localhost:8082")
	v.SetDefault("scheduler_interval", "30s")
	v.SetDefault("http_timeout", "10s")
	v.SetDefault("grpc_timeout", "3s")
	v.SetDefault("github_base_url", "https://api.github.com")
	v.SetDefault("stack_base_url", "https://api.stackexchange.com/2.3")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.ScrapperGRPCAddr = strings.TrimSpace(cfg.ScrapperGRPCAddr)
	cfg.BotGRPCAddr = strings.TrimSpace(cfg.BotGRPCAddr)
	cfg.GitHubBaseURL = strings.TrimSpace(cfg.GitHubBaseURL)
	cfg.StackBaseURL = strings.TrimSpace(cfg.StackBaseURL)

	if cfg.ScrapperGRPCAddr == "" {
		return nil, fmt.Errorf("SCRAPPER_GRPC_ADDR is required")
	}
	if cfg.BotGRPCAddr == "" {
		return nil, fmt.Errorf("BOT_GRPC_ADDR is required")
	}
	if cfg.SchedulerInterval <= 0 {
		return nil, fmt.Errorf("SCHEDULER_INTERVAL must be positive")
	}
	if cfg.HTTPTimeout <= 0 {
		return nil, fmt.Errorf("HTTP_TIMEOUT must be positive")
	}
	if cfg.GRPCTimeout <= 0 {
		return nil, fmt.Errorf("GRPC_TIMEOUT must be positive")
	}
	if cfg.GitHubBaseURL == "" || cfg.StackBaseURL == "" {
		return nil, fmt.Errorf("GITHUB_BASE_URL and STACK_BASE_URL are required")
	}

	return &cfg, nil
}
