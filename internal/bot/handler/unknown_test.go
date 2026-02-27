package handler_test

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/handler"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/memory"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/usecase"
)

func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		update       tgbotapi.Update
		wantContains []string
		wantChatID   int64
	}{
		{
			name:   "неизвестная команда /foo",
			update: makeCommandUpdate(100, "alice", "foo"),
			wantContains: []string{
				"Неизвестная команда",
				"/help",
			},
			wantChatID: 100,
		},
		{
			name:   "неизвестная команда /settings",
			update: makeCommandUpdate(200, "bob", "settings"),
			wantContains: []string{
				"Неизвестная команда",
				"/help",
			},
			wantChatID: 200,
		},
		{
			name:   "обычный текст обрабатывается как неизвестная команда",
			update: makeTextUpdate(300, "carol", "привет"),
			wantContains: []string{
				"Неизвестная команда",
				"/help",
			},
			wantChatID: 300,
		},
		{
			name:   "текст с цифрами обрабатывается как неизвестная команда",
			update: makeTextUpdate(400, "dave", "12345"),
			wantContains: []string{
				"Неизвестная команда",
			},
			wantChatID: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockBotClient{}
			repo := memory.NewUserRepository()
			uc := usecase.NewUserUseCase(repo)
			h := handler.New(mock, uc, newTestLogger())

			h.HandleUpdate(tt.update)

			require.Len(t, mock.messages, 1)
			assert.Equal(t, tt.wantChatID, mock.messages[0].ChatID)

			for _, s := range tt.wantContains {
				assert.Contains(t, mock.messages[0].Text, s)
			}
		})
	}
}
