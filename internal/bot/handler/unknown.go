package handler

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) handleUnknown(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID

	h.logger.Warn("unknown command received",
		slog.Int64("chat_id", chatID),
		slog.String("text", update.Message.Text),
	)

	h.sendMessage(chatID, "Неизвестная команда. Воспользуйтесь /help, чтобы посмотреть список доступных команд.")
}
