package main

import (
	"log/slog"
	"os"

	loggerpkg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/bot/app"
)

func main() {
	logger := loggerpkg.NewJSON(os.Stdout, slog.LevelInfo)
	if err := app.Run(logger); err != nil {
		logger.Error("bot app failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
