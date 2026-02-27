package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api *tgbotapi.BotAPI
}

func New(token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{api: api}, nil
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) SetCommands(commands map[string]string) error {
	botCommands := make([]tgbotapi.BotCommand, 0, len(commands))
	for name, desc := range commands {
		botCommands = append(botCommands, tgbotapi.BotCommand{
			Command:     name,
			Description: desc,
		})
	}

	_, err := b.api.Request(tgbotapi.NewSetMyCommands(botCommands...))
	return err
}

func (b *Bot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return b.api.GetUpdatesChan(config)
}

func (b *Bot) StopReceivingUpdates() {
	b.api.StopReceivingUpdates()
}

func (b *Bot) GetUserName() string {
	return b.api.Self.UserName
}
