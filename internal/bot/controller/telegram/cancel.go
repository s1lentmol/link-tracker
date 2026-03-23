package handler

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func newCancelCommand(h *Handler) command {
	return command{
		name:        "cancel",
		description: "Отменить активное действие",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			state := h.stateStore.Get(chatID)
			if !state.Active() {
				h.sendMessage(chatID, "Нет активного действия для отмены.")
				return
			}

			h.stateStore.Clear(chatID)
			h.sendMessage(chatID, "Действие отменено.")
		},
	}
}
