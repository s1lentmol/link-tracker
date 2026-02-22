package handler_test

import (
	"io"
	"log/slog"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/handler"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/memory"
)

type sentMessage struct {
	ChatID int64
	Text   string
}

type mockBotClient struct {
	messages     []sentMessage
	requestCalls int
}

func (m *mockBotClient) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		m.messages = append(m.messages, sentMessage{
			ChatID: msg.ChatID,
			Text:   msg.Text,
		})
	}

	return tgbotapi.Message{}, nil
}

func (m *mockBotClient) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.requestCalls++

	return &tgbotapi.APIResponse{Ok: true}, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func makeCommandUpdate(chatID int64, username, command string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/" + command,
			Chat: &tgbotapi.Chat{ID: chatID},
			From: &tgbotapi.User{UserName: username},
			Entities: []tgbotapi.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: len("/" + command),
				},
			},
		},
	}
}

func makeTextUpdate(chatID int64, username, text string) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: text,
			Chat: &tgbotapi.Chat{ID: chatID},
			From: &tgbotapi.User{UserName: username},
		},
	}
}

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		update       tgbotapi.Update
		wantContains string
		wantChatID   int64
		wantNoSend   bool
	}{
		{
			name:         "команда /start маршрутизируется к start-обработчику",
			update:       makeCommandUpdate(100, "alice", "start"),
			wantContains: "Добро пожаловать",
			wantChatID:   100,
		},
		{
			name:         "команда /help маршрутизируется к help-обработчику",
			update:       makeCommandUpdate(200, "bob", "help"),
			wantContains: "Доступные команды",
			wantChatID:   200,
		},
		{
			name:         "неизвестная команда маршрутизируется к unknown-обработчику",
			update:       makeCommandUpdate(300, "carol", "nonexistent"),
			wantContains: "Неизвестная команда",
			wantChatID:   300,
		},
		{
			name:         "обычный текст маршрутизируется к unknown-обработчику",
			update:       makeTextUpdate(400, "dave", "привет"),
			wantContains: "Неизвестная команда",
			wantChatID:   400,
		},
		{
			name:       "nil message игнорируется",
			update:     tgbotapi.Update{Message: nil},
			wantNoSend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockBotClient{}
			repo := memory.NewUserRepository()
			h := handler.New(mock, repo, newTestLogger())

			h.HandleUpdate(tt.update)

			if tt.wantNoSend {
				assert.Empty(t, mock.messages)
				return
			}

			require.Len(t, mock.messages, 1)
			assert.Equal(t, tt.wantChatID, mock.messages[0].ChatID)
			assert.Contains(t, mock.messages[0].Text, tt.wantContains)
		})
	}
}

func TestSetMyCommands(t *testing.T) {
	t.Parallel()

	mock := &mockBotClient{}
	repo := memory.NewUserRepository()
	h := handler.New(mock, repo, newTestLogger())

	h.SetMyCommands()

	assert.Equal(t, 1, mock.requestCalls, "SetMyCommands должен вызвать Request один раз")
}

func TestAllCommands(t *testing.T) {
	t.Parallel()

	mock := &mockBotClient{}
	repo := memory.NewUserRepository()
	h := handler.New(mock, repo, newTestLogger())

	cmds := h.AllCommands()

	assert.Len(t, cmds, 2, "должно быть 2 зарегистрированные команды")

	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}

	assert.True(t, names["start"], "должна быть команда start")
	assert.True(t, names["help"], "должна быть команда help")
}
