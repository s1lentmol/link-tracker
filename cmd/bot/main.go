package main

import (
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/app"
	loggerpkg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/logger"
)

func main() {
	logger := loggerpkg.NewJSON(os.Stdout, slog.LevelInfo)
	if err := app.Run(logger); err != nil {
		logger.Error("bot app failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
