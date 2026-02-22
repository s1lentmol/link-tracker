package handler

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain/model"
)

func newStartCommand(h *Handler) Command {
	return Command{
		Name:        "start",
		Description: "Начало работы с ботом",
		Handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			username := update.Message.From.UserName

			exists, err := h.userRepo.Exists(chatID)
			if err != nil {
				h.logger.Error("failed to check user existence",
					slog.Int64("chat_id", chatID),
					slog.String("error", err.Error()),
				)
			}

			if !exists {
				user := model.User{
					ChatID:   chatID,
					Username: username,
				}

				if err := h.userRepo.Save(user); err != nil {
					h.logger.Error("failed to save user",
						slog.Int64("chat_id", chatID),
						slog.String("error", err.Error()),
					)
				}

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
