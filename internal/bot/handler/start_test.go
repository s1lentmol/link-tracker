package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/handler"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/memory"
)

func TestStartCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		chatID       int64
		username     string
		callCount    int
		wantContains []string
	}{
		{
			name:      "приветствие содержит имя пользователя и подсказку /help",
			chatID:    100,
			username:  "alice",
			callCount: 1,
			wantContains: []string{
				"Добро пожаловать, alice",
				"/help",
			},
		},
		{
			name:      "приветствие для другого пользователя",
			chatID:    200,
			username:  "bob_test",
			callCount: 1,
			wantContains: []string{
				"Добро пожаловать, bob_test",
				"/help",
			},
		},
		{
			name:      "приветствие для пользователя с пустым именем",
			chatID:    300,
			username:  "",
			callCount: 1,
			wantContains: []string{
				"Добро пожаловать, ",
			},
		},
		{
			name:      "повторный /start не дублирует пользователя",
			chatID:    400,
			username:  "carol",
			callCount: 2,
			wantContains: []string{
				"Добро пожаловать, carol",
			},
		},
		{
			name:      "три вызова /start — бот отвечает три раза",
			chatID:    500,
			username:  "dave",
			callCount: 3,
			wantContains: []string{
				"Добро пожаловать, dave",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockBotClient{}
			repo := memory.NewUserRepository()
			h := handler.New(mock, repo, newTestLogger())

			update := makeCommandUpdate(tt.chatID, tt.username, "start")
			for range tt.callCount {
				h.HandleUpdate(update)
			}

			require.Len(t, mock.messages, tt.callCount)

			for _, s := range tt.wantContains {
				assert.Contains(t, mock.messages[0].Text, s)
			}

			assert.Equal(t, tt.chatID, mock.messages[0].ChatID)

			exists, err := repo.Exists(tt.chatID)
			require.NoError(t, err)
			assert.True(t, exists)

			user, err := repo.FindByChatID(tt.chatID)
			require.NoError(t, err)
			assert.Equal(t, tt.chatID, user.ChatID)
			assert.Equal(t, tt.username, user.Username)
		})
	}
}
