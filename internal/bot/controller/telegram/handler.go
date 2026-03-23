package handler

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	grpcadapter "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/infrastructure/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/shared/pb"
)

type Bot interface {
	SendMessage(chatID int64, text string) error
	SetCommands(commands map[string]string) error
}

type UserService interface {
	RegisterUser(chatID int64, username string) (bool, error)
}

type ScrapperService interface {
	RegisterChat(chatID int64) error
	DeleteChat(chatID int64) error
	AddLink(chatID int64, link string, tags []string, filters []string) (*pb.LinkResponse, error)
	RemoveLink(chatID int64, link string) (*pb.LinkResponse, error)
	ListLinks(chatID int64) (*pb.ListLinksResponse, error)
}

type command struct {
	name        string
	description string
	handle      func(update tgbotapi.Update)
}

type Option func(h *Handler)

func WithScrapperService(svc ScrapperService) Option {
	return func(h *Handler) {
		h.scrapper = svc
	}
}

func WithStateStore(store StateStore) Option {
	return func(h *Handler) {
		h.stateStore = store
	}
}

type Handler struct {
	bot         Bot
	logger      *slog.Logger
	commands    map[string]command
	userService UserService
	scrapper    ScrapperService
	stateStore  StateStore
}

func New(bot Bot, userService UserService, logger *slog.Logger, opts ...Option) *Handler {
	h := &Handler{
		bot:         bot,
		logger:      logger,
		commands:    make(map[string]command),
		userService: userService,
		scrapper:    noopScrapperService{},
		stateStore:  NewMemoryStateStore(),
	}

	for _, opt := range opts {
		opt(h)
	}

	h.registerCommands()
	h.setMyCommands()

	return h
}

func (h *Handler) registerCommands() {
	cmds := []command{
		newStartCommand(h),
		newHelpCommand(h),
		newTrackCommand(h),
		newUntrackCommand(h),
		newListCommand(h),
		newCancelCommand(h),
	}

	for _, cmd := range cmds {
		h.commands[cmd.name] = cmd
	}
}

func (h *Handler) setMyCommands() {
	cmds := make(map[string]string)
	for name, cmd := range h.commands {
		cmds[name] = cmd.description
	}

	if err := h.bot.SetCommands(cmds); err != nil {
		h.logger.Error("failed to set bot commands", slog.String("error", err.Error()))
	} else {
		h.logger.Info("bot commands registered", slog.Int("count", len(cmds)))
	}
}

func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	state := h.stateStore.Get(chatID)

	if update.Message.IsCommand() {
		cmdName := update.Message.Command()

		if state.Active() {
			if cmdName == "cancel" {
				h.stateStore.Clear(chatID)
				h.sendMessage(chatID, "Действие отменено.")
				return
			}

			h.stateStore.Clear(chatID)
			h.sendMessage(chatID, "Процесс отслеживания отменён.")
		}

		cmd, ok := h.commands[cmdName]
		if !ok {
			h.handleUnknown(update)
			return
		}

		username := ""
		if update.Message.From != nil {
			username = update.Message.From.UserName
		}

		h.logger.Info("handling command",
			slog.String("command", cmdName),
			slog.Int64("chat_id", update.Message.Chat.ID),
			slog.String("username", username),
		)

		cmd.handle(update)
		return
	}

	if state.Active() {
		h.handleDialogInput(update, state)
		return
	}

	h.handleUnknown(update)
}

func (h *Handler) sendMessage(chatID int64, text string) {
	if err := h.bot.SendMessage(chatID, text); err != nil {
		h.logger.Error("failed to send message",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()),
		)
	}
}

func statusCode(err error) string {
	return grpcadapter.StatusCode(err).String()
}

type noopScrapperService struct{}

func (noopScrapperService) RegisterChat(chatID int64) error {
	return nil
}

func (noopScrapperService) DeleteChat(chatID int64) error {
	return nil
}

func (noopScrapperService) AddLink(chatID int64, link string, tags []string, filters []string) (*pb.LinkResponse, error) {
	return &pb.LinkResponse{Id: 1, Url: link, Tags: tags, Filters: filters}, nil
}

func (noopScrapperService) RemoveLink(chatID int64, link string) (*pb.LinkResponse, error) {
	return &pb.LinkResponse{Id: 1, Url: link}, nil
}

func (noopScrapperService) ListLinks(chatID int64) (*pb.ListLinksResponse, error) {
	return &pb.ListLinksResponse{}, nil
}
