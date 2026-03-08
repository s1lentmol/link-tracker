package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

func NewViperFromEnvFile(path string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	v.AutomaticEnv()
	return v, nil
}
