package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/storage"
)

func TestRepository_RegisterChat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chatID  int64
		setup   func(r *storage.Repository)
		wantErr error
	}{
		{
			name:   "успешная регистрация чата",
			chatID: 1,
		},
		{
			name:   "повторная регистрация возвращает ошибку",
			chatID: 1,
			setup: func(r *storage.Repository) {
				require.NoError(t, r.RegisterChat(1))
			},
			wantErr: apperr.ErrChatExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := storage.NewRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			err := repo.RegisterChat(tt.chatID)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRepository_AddListRemoveLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		chatID     int64
		link       string
		tags       []string
		filters    []string
		setup      func(r *storage.Repository)
		wantAddErr error
	}{
		{
			name:    "успешное добавление и удаление ссылки",
			chatID:  1,
			link:    "https://github.com/owner/repo",
			tags:    []string{"work"},
			filters: []string{"type=pr"},
			setup: func(r *storage.Repository) {
				require.NoError(t, r.RegisterChat(1))
			},
		},
		{
			name:       "добавление в несуществующий чат",
			chatID:     1,
			link:       "https://github.com/owner/repo",
			wantAddErr: apperr.ErrChatNotFound,
		},
		{
			name:   "повторное добавление ссылки",
			chatID: 1,
			link:   "https://github.com/owner/repo",
			setup: func(r *storage.Repository) {
				require.NoError(t, r.RegisterChat(1))
				_, err := r.AddLink(1, "https://github.com/owner/repo", nil, nil)
				require.NoError(t, err)
			},
			wantAddErr: apperr.ErrLinkExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := storage.NewRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			sub, err := repo.AddLink(tt.chatID, tt.link, tt.tags, tt.filters)
			if tt.wantAddErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantAddErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sub)
			assert.Equal(t, tt.link, sub.URL)
			assert.Equal(t, tt.tags, sub.Tags)
			assert.Equal(t, tt.filters, sub.Filters)

			list, err := repo.ListLinks(tt.chatID)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, tt.link, list[0].URL)

			removed, err := repo.RemoveLink(tt.chatID, tt.link)
			require.NoError(t, err)
			assert.Equal(t, tt.link, removed.URL)

			list, err = repo.ListLinks(tt.chatID)
			require.NoError(t, err)
			assert.Empty(t, list)
		})
	}
}

func TestRepository_RemoveLinkErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chatID  int64
		link    string
		setup   func(r *storage.Repository)
		wantErr error
	}{
		{
			name:    "удаление в несуществующем чате",
			chatID:  999,
			link:    "https://github.com/owner/repo",
			wantErr: apperr.ErrChatNotFound,
		},
		{
			name:   "удаление несуществующей ссылки",
			chatID: 1,
			link:   "https://github.com/owner/repo",
			setup: func(r *storage.Repository) {
				require.NoError(t, r.RegisterChat(1))
			},
			wantErr: apperr.ErrLinkNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := storage.NewRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			_, err := repo.RemoveLink(tt.chatID, tt.link)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
