package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	memory "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/storage"
)

func TestUserRepository_Save(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user domain.User
	}{
		{
			name: "сохранение пользователя с положительным chatID",
			user: domain.User{ChatID: 12345, Username: "alice"},
		},
		{
			name: "сохранение пользователя с нулевым chatID",
			user: domain.User{ChatID: 0, Username: "zero"},
		},
		{
			name: "сохранение пользователя с пустым именем",
			user: domain.User{ChatID: 99999, Username: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := memory.NewUserRepository()

			err := repo.Save(tt.user)
			require.NoError(t, err)

			found, err := repo.FindByChatID(tt.user.ChatID)
			require.NoError(t, err)
			assert.Equal(t, tt.user.ChatID, found.ChatID)
			assert.Equal(t, tt.user.Username, found.Username)
		})
	}
}

func TestUserRepository_FindByChatID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     []domain.User
		chatID    int64
		wantUser  *domain.User
		wantError bool
	}{
		{
			name:     "поиск существующего пользователя",
			setup:    []domain.User{{ChatID: 100, Username: "alice"}},
			chatID:   100,
			wantUser: &domain.User{ChatID: 100, Username: "alice"},
		},
		{
			name:      "поиск несуществующего пользователя возвращает ошибку",
			setup:     []domain.User{},
			chatID:    999,
			wantError: true,
		},
		{
			name: "поиск среди нескольких пользователей",
			setup: []domain.User{
				{ChatID: 1, Username: "one"},
				{ChatID: 2, Username: "two"},
				{ChatID: 3, Username: "three"},
			},
			chatID:   2,
			wantUser: &domain.User{ChatID: 2, Username: "two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := memory.NewUserRepository()
			for _, u := range tt.setup {
				require.NoError(t, repo.Save(u))
			}

			found, err := repo.FindByChatID(tt.chatID)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, found)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser.ChatID, found.ChatID)
				assert.Equal(t, tt.wantUser.Username, found.Username)
			}
		})
	}
}

func TestUserRepository_Exists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    []domain.User
		chatID   int64
		expected bool
	}{
		{
			name:     "существующий пользователь возвращает true",
			setup:    []domain.User{{ChatID: 100, Username: "alice"}},
			chatID:   100,
			expected: true,
		},
		{
			name:     "несуществующий пользователь возвращает false",
			setup:    []domain.User{},
			chatID:   999,
			expected: false,
		},
		{
			name: "возвращает true только для совпадающего chatID",
			setup: []domain.User{
				{ChatID: 1, Username: "one"},
				{ChatID: 2, Username: "two"},
			},
			chatID:   1,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := memory.NewUserRepository()
			for _, u := range tt.setup {
				require.NoError(t, repo.Save(u))
			}

			exists, err := repo.Exists(tt.chatID)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

func TestUserRepository_SaveOverwrite(t *testing.T) {
	t.Parallel()

	repo := memory.NewUserRepository()

	require.NoError(t, repo.Save(domain.User{ChatID: 100, Username: "old_name"}))
	require.NoError(t, repo.Save(domain.User{ChatID: 100, Username: "new_name"}))

	user, err := repo.FindByChatID(100)
	require.NoError(t, err)
	assert.Equal(t, "new_name", user.Username, "повторное сохранение должно перезаписать данные")
}
