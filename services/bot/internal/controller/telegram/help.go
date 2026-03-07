package handler

import (
	"fmt"
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newHelpCommand(h *Handler) command {
	return command{
		name:        "help",
		description: "Список доступных команд",
		handle: func(update tgbotapi.Update) {
			var sb strings.Builder

			sb.WriteString("Доступные команды:\n\n")

			cmdNames := make([]string, 0, len(h.commands))
			for name := range h.commands {
				cmdNames = append(cmdNames, name)
			}
			sort.Strings(cmdNames)

			for _, name := range cmdNames {
				cmd := h.commands[name]
				fmt.Fprintf(&sb, "/%s — %s\n", cmd.name, cmd.description)
			}

			h.sendMessage(update.Message.Chat.ID, sb.String())
		},
	}
}
