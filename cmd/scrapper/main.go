package main

import (
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/app"
	loggerpkg "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/pkg/logger"
)

func main() {
	logger := loggerpkg.NewJSON(os.Stdout, slog.LevelInfo)
	if err := app.Run(logger); err != nil {
		logger.Error("scrapper app failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
