package handler

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/infrastructure/grpc"
)

var stackOverflowQuestionPath = regexp.MustCompile(`^/questions/(\d+)(/.*)?$`)

const (
	githubHost        = "github.com"
	stackOverflowHost = "stackoverflow.com"
)

func (h *Handler) handleDialogInput(update tgbotapi.Update, state DialogState) {
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	switch state.Step {
	case StepAwaitURL:
		if _, err := validateSupportedURL(text); err != nil {
			h.logger.Warn("invalid track url input",
				slog.Int64("chat_id", chatID),
				slog.String("input", text),
				slog.String("error", err.Error()),
			)
			h.sendMessage(chatID, "Некорректная ссылка. Поддерживаются GitHub репозитории и вопросы StackOverflow.")
			return
		}

		h.stateStore.Set(chatID, DialogState{Step: StepAwaitTags, Link: text})
		h.sendMessage(chatID, "Введите теги через запятую (или отправьте '-' для пропуска).")
	case StepAwaitTags:
		tags := parseListInput(text)
		h.stateStore.Set(chatID, DialogState{Step: StepAwaitFilters, Link: state.Link, Tags: tags})
		h.sendMessage(chatID, "Введите фильтры через запятую (или отправьте '-' для пропуска).")
	case StepAwaitFilters:
		filters := parseListInput(text)
		h.finishTrack(chatID, state.Link, state.Tags, filters)
		h.stateStore.Clear(chatID)
	case StepAwaitUntrackURL:
		if _, err := validateSupportedURL(text); err != nil {
			h.logger.Warn("invalid untrack url input",
				slog.Int64("chat_id", chatID),
				slog.String("input", text),
				slog.String("error", err.Error()),
			)
			h.sendMessage(chatID, "Некорректная ссылка. Поддерживаются GitHub репозитории и вопросы StackOverflow.")
			return
		}

		h.executeUntrack(chatID, text)
		h.stateStore.Clear(chatID)
	default:
		h.stateStore.Clear(chatID)
	}
}

func (h *Handler) finishTrack(chatID int64, link string, tags []string, filters []string) {
	_, err := h.scrapper.AddLink(chatID, link, tags, filters)
	if err != nil {
		h.logger.Warn("failed to add tracked link",
			slog.Int64("chat_id", chatID),
			slog.String("url", link),
			slog.String("status", statusCode(err)),
			slog.String("error", err.Error()),
		)

		switch grpcadapter.StatusCode(err) {
		case codes.AlreadyExists:
			h.sendMessage(chatID, "Ссылка уже отслеживается")
		case codes.NotFound:
			h.sendMessage(chatID, "Сначала выполните /start, чтобы зарегистрировать чат.")
		default:
			h.sendMessage(chatID, "Не удалось добавить ссылку. Попробуйте позже.")
		}

		return
	}

	h.sendMessage(chatID, fmt.Sprintf("Ссылка добавлена в отслеживание: %s", link))
}

func validateSupportedURL(raw string) (*url.URL, error) {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	switch strings.ToLower(u.Host) {
	case githubHost:
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid github link")
		}
	case stackOverflowHost:
		if !stackOverflowQuestionPath.MatchString(u.Path) {
			return nil, fmt.Errorf("invalid stackoverflow link")
		}
	default:
		return nil, fmt.Errorf("unsupported host: %s", u.Host)
	}

	return u, nil
}

func parseListInput(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		item := strings.TrimSpace(p)
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}
