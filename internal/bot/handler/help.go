package handler

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newHelpCommand(h *Handler) Command {
	return Command{
		Name:        "help",
		Description: "Список доступных команд",
		Handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID

			var sb strings.Builder

			sb.WriteString("Доступные команды:\n\n")

			for _, cmd := range h.AllCommands() {
				fmt.Fprintf(&sb, "/%s — %s\n", cmd.Name, cmd.Description)
			}

			h.sendMessage(chatID, sb.String())
		},
	}
}
