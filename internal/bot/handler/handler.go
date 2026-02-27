package handler

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot interface {
	SendMessage(chatID int64, text string) error
	SetCommands(commands map[string]string) error
}

type UserService interface {
	RegisterUser(chatID int64, username string) (bool, error)
}

type command struct {
	name        string
	description string
	handle      func(update tgbotapi.Update)
}

type Handler struct {
	bot         Bot
	logger      *slog.Logger
	commands    map[string]command
	userService UserService
}

func New(bot Bot, userService UserService, logger *slog.Logger) *Handler {
	h := &Handler{
		bot:         bot,
		logger:      logger,
		commands:    make(map[string]command),
		userService: userService,
	}

	h.registerCommands()
	h.setMyCommands()

	return h
}

func (h *Handler) registerCommands() {
	cmds := []command{
		newStartCommand(h),
		newHelpCommand(h),
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

	if !update.Message.IsCommand() {
		h.handleUnknown(update)
		return
	}

	cmdName := update.Message.Command()

	cmd, ok := h.commands[cmdName]
	if !ok {
		h.handleUnknown(update)
		return
	}

	h.logger.Info("handling command",
		slog.String("command", cmdName),
		slog.Int64("chat_id", update.Message.Chat.ID),
		slog.String("username", update.Message.From.UserName),
	)

	cmd.handle(update)
}

func (h *Handler) sendMessage(chatID int64, text string) {
	if err := h.bot.SendMessage(chatID, text); err != nil {
		h.logger.Error("failed to send message",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()),
		)
	}
}
