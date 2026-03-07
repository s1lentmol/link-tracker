package handler

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newStartCommand(h *Handler) command {
	return command{
		name:        "start",
		description: "Начало работы с ботом",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			username := update.Message.From.UserName

			isNew, err := h.userService.RegisterUser(chatID, username)
			if err != nil {
				h.logger.Error("failed to register user",
					slog.Int64("chat_id", chatID),
					slog.String("error", err.Error()),
				)
			}

			if isNew {
				h.logger.Info("new user registered",
					slog.Int64("chat_id", chatID),
					slog.String("username", username),
				)
			}

			text := fmt.Sprintf(
				"Добро пожаловать, %s! Используйте /help, чтобы посмотреть доступные команды.",
				username,
			)

			h.sendMessage(chatID, text)
		},
	}
}
