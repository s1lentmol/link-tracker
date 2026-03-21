package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	handler "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/bot/controller/telegram"
)

func TestHelpCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		chatID       int64
		username     string
		wantContains []string
	}{
		{
			name:     "ответ содержит заголовок и команды в формате /name — description",
			chatID:   100,
			username: "alice",
			wantContains: []string{
				"Доступные команды",
				"/start — Начало работы с ботом",
				"/help — Список доступных команд",
			},
		},
		{
			name:     "ответ отправляется в правильный чат",
			chatID:   999,
			username: "bob",
			wantContains: []string{
				"Доступные команды",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockBotClient{}
			h := handler.New(mock, newTestLogger())

			update := makeCommandUpdate(tt.chatID, tt.username, "help")
			h.HandleUpdate(update)

			require.Len(t, mock.messages, 1)
			assert.Equal(t, tt.chatID, mock.messages[0].ChatID)

			for _, s := range tt.wantContains {
				assert.Contains(t, mock.messages[0].Text, s)
			}
		})
	}
}
