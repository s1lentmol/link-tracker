package handler

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain/repository"
)

type BotClient interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

type Command struct {
	Name        string
	Description string
	Handle      func(update tgbotapi.Update)
}

type Handler struct {
	bot      BotClient
	logger   *slog.Logger
	commands map[string]Command
	userRepo repository.UserRepository
}

func New(bot BotClient, userRepo repository.UserRepository, logger *slog.Logger) *Handler {
	h := &Handler{
		bot:      bot,
		logger:   logger,
		commands: make(map[string]Command),
		userRepo: userRepo,
	}

	h.registerCommands()

	return h
}

func (h *Handler) registerCommands() {
	cmds := []Command{
		newStartCommand(h),
		newHelpCommand(h),
	}

	for _, cmd := range cmds {
		h.commands[cmd.Name] = cmd
	}
}

func (h *Handler) SetMyCommands() {
	var botCommands []tgbotapi.BotCommand

	for _, cmd := range h.commands {
		botCommands = append(botCommands, tgbotapi.BotCommand{
			Command:     cmd.Name,
			Description: cmd.Description,
		})
	}

	cfg := tgbotapi.NewSetMyCommands(botCommands...)

	if _, err := h.bot.Request(cfg); err != nil {
		h.logger.Error("failed to set bot commands", slog.String("error", err.Error()))
	} else {
		h.logger.Info("bot commands registered", slog.Int("count", len(botCommands)))
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

	cmd.Handle(update)
}

func (h *Handler) AllCommands() []Command {
	cmds := make([]Command, 0, len(h.commands))
	for _, cmd := range h.commands {
		cmds = append(cmds, cmd)
	}

	return cmds
}

func (h *Handler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if _, err := h.bot.Send(msg); err != nil {
		h.logger.Error("failed to send message",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()),
		)
	}
}
