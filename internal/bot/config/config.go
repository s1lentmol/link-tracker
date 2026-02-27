package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppTelegramToken string `mapstructure:"app_telegram_token"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	_ = v.ReadInConfig()

	v.AutomaticEnv()
	v.BindEnv("app_telegram_token")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.AppTelegramToken = strings.TrimSpace(cfg.AppTelegramToken)
	if cfg.AppTelegramToken == "" {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN is required")
	}

	return &cfg, nil
}
