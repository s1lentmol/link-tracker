package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/infrastructure/grpc"
)

func newUntrackCommand(h *Handler) command {
	return command{
		name:        "untrack",
		description: "Прекратить отслеживание ссылки",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			arg := strings.TrimSpace(update.Message.CommandArguments())
			if arg == "" {
				h.stateStore.Set(chatID, DialogState{Step: StepAwaitUntrackURL})
				h.sendMessage(chatID, "Отправьте ссылку, которую нужно удалить из отслеживания.")
				return
			}

			h.executeUntrack(chatID, arg)
		},
	}
}

func (h *Handler) executeUntrack(chatID int64, link string) {
	if err := validateSupportedURL(link); err != nil {
		h.sendMessage(chatID, "Некорректная ссылка. Поддерживаются GitHub репозитории и вопросы StackOverflow.")
		return
	}

	_, err := h.scrapper.RemoveLink(chatID, link)
	if err != nil {
		h.logger.Warn("failed to untrack link",
			slog.Int64("chat_id", chatID),
			slog.String("url", link),
			slog.String("status", statusCode(err)),
			slog.String("error", err.Error()),
		)

		switch grpcadapter.StatusCode(err) {
		case codes.NotFound:
			h.sendMessage(chatID, "Ссылка не найдена в отслеживаемых.")
		case codes.OK,
			codes.Canceled,
			codes.Unknown,
			codes.InvalidArgument,
			codes.DeadlineExceeded,
			codes.AlreadyExists,
			codes.PermissionDenied,
			codes.ResourceExhausted,
			codes.FailedPrecondition,
			codes.Aborted,
			codes.OutOfRange,
			codes.Unimplemented,
			codes.Internal,
			codes.Unavailable,
			codes.DataLoss,
			codes.Unauthenticated:
			h.sendMessage(chatID, "Не удалось удалить ссылку. Попробуйте позже.")
		default:
			h.sendMessage(chatID, "Не удалось удалить ссылку. Попробуйте позже.")
		}
		return
	}

	h.sendMessage(chatID, fmt.Sprintf("Ссылка удалена из отслеживания: %s", link))
}
