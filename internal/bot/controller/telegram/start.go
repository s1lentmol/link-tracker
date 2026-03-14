package handler

import (
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/infrastructure/grpc"
)

func newStartCommand(h *Handler) command {
	return command{
		name:        "start",
		description: "Начало работы с ботом",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			username := ""
			if update.Message.From != nil {
				username = update.Message.From.UserName
			}

			if h.userService != nil {
				_, err := h.userService.RegisterUser(chatID, username)
				if err != nil {
					h.logger.Error("failed to register user locally",
						slog.Int64("chat_id", chatID),
						slog.String("error", err.Error()),
					)
				}
			}

			err := h.scrapper.RegisterChat(chatID)
			if err != nil && grpcadapter.StatusCode(err) != codes.AlreadyExists {
				h.logger.Error("failed to register chat in scrapper",
					slog.Int64("chat_id", chatID),
					slog.String("status", statusCode(err)),
					slog.String("error", err.Error()),
				)
				h.sendMessage(chatID, "Не удалось зарегистрировать чат в scrapper. Попробуйте позже.")
				return
			}

			text := fmt.Sprintf(
				"Добро пожаловать, %s! Используйте /help, чтобы посмотреть доступные команды.",
				username,
			)

			h.sendMessage(chatID, text)
		},
	}
}
