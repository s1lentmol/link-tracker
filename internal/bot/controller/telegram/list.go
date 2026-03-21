package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/infrastructure/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

func newListCommand(h *Handler) command {
	return command{
		name:        "list",
		description: "Показать отслеживаемые ссылки",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			tag := strings.TrimSpace(update.Message.CommandArguments())

			resp, err := h.scrapper.ListLinks(chatID)
			if err != nil {
				h.logger.Warn("failed to list links",
					slog.Int64("chat_id", chatID),
					slog.String("status", statusCode(err)),
					slog.String("error", err.Error()),
				)
				switch grpcadapter.StatusCode(err) {
				case codes.NotFound:
					h.sendMessage(chatID, "Сначала выполните /start, чтобы зарегистрировать чат.")
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
					h.sendMessage(chatID, "Не удалось получить список ссылок. Попробуйте позже.")
				default:
					h.sendMessage(chatID, "Не удалось получить список ссылок. Попробуйте позже.")
				}
				return
			}

			links := filterLinksByTag(resp.GetLinks(), tag)
			if len(links) == 0 {
				h.sendMessage(chatID, "Список отслеживаемых ссылок пуст.")
				return
			}

			var sb strings.Builder
			sb.WriteString(formatTrackedLinks(links))
			if tag != "" {
				_, _ = fmt.Fprintf(&sb, "\n\nФильтр по тегу: %s", tag)
			}

			h.sendMessage(chatID, sb.String())
		},
	}
}

func filterLinksByTag(links []*pb.LinkResponse, tag string) []*pb.LinkResponse {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return links
	}

	filtered := make([]*pb.LinkResponse, 0, len(links))
	for _, link := range links {
		for _, t := range link.GetTags() {
			if t == tag {
				filtered = append(filtered, link)
				break
			}
		}
	}
	return filtered
}

func formatTrackedLinks(links []*pb.LinkResponse) string {
	var sb strings.Builder
	sb.WriteString("Отслеживаемые ссылки:\n")
	for _, link := range links {
		sb.WriteString("\n- ")
		sb.WriteString(link.GetUrl())
		if len(link.GetTags()) > 0 {
			sb.WriteString("\n  теги: ")
			sb.WriteString(strings.Join(link.GetTags(), ", "))
		}
		if len(link.GetFilters()) > 0 {
			sb.WriteString("\n  фильтры: ")
			sb.WriteString(strings.Join(link.GetFilters(), ", "))
		}
	}
	return sb.String()
}
