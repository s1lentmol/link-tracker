package logger

import (
	"io"
	"log/slog"
)

func NewJSON(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
