package handler

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func newTrackCommand(h *Handler) command {
	return command{
		name:        "track",
		description: "Начать отслеживание ссылки",
		handle: func(update tgbotapi.Update) {
			chatID := update.Message.Chat.ID
			h.stateStore.Set(chatID, DialogState{Step: StepAwaitURL})
			h.sendMessage(chatID, "Отправьте ссылку, которую нужно отслеживать.")
		},
	}
}
