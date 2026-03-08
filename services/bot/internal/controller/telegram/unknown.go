package handler

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) handleUnknown(update tgbotapi.Update) {
	username := ""
	if update.Message != nil && update.Message.From != nil {
		username = update.Message.From.UserName
	}

	h.logger.Warn("unknown command received",
		slog.Int64("chat_id", update.Message.Chat.ID),
		slog.String("username", username),
		slog.String("text", update.Message.Text),
	)

	h.sendMessage(update.Message.Chat.ID, "Неизвестная команда. Воспользуйтесь /help, чтобы посмотреть список доступных команд.")
}
