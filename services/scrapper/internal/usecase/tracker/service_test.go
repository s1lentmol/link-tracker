package tracker_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ghclient "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/scrapper/internal/adapter/http/github"
	stackclient "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/scrapper/internal/adapter/http/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/scrapper/internal/adapter/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/services/scrapper/internal/usecase/tracker"
)

type notifierCall struct {
	ID          int64
	URL         string
	Description string
	ChatIDs     []int64
}

type notifierMock struct {
	mu    sync.Mutex
	calls []notifierCall
}

func (m *notifierMock) SendUpdate(id int64, url string, description string, chatIDs []int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, notifierCall{
		ID:          id,
		URL:         url,
		Description: description,
		ChatIDs:     append([]int64(nil), chatIDs...),
	})
	return nil
}

func (m *notifierMock) Calls() []notifierCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifierCall(nil), m.calls...)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestService_ValidateURL(t *testing.T) {
	t.Parallel()

	svc := tracker.New(storage.NewRepository(), nil, nil, &notifierMock{}, newTestLogger())

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "валидный github url", url: "https://github.com/owner/repo"},
		{name: "валидный stackoverflow url", url: "https://stackoverflow.com/questions/123/title"},
		{name: "невалидная схема", url: "tbank://github.com/owner/repo", wantErr: true},
		{name: "неподдерживаемый хост", url: "https://example.com/a/b", wantErr: true},
		{name: "невалидный github путь", url: "https://github.com/owner", wantErr: true},
		{name: "невалидный stackoverflow путь", url: "https://stackoverflow.com/questions", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := svc.ValidateURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_CheckUpdates_GitHub(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	updatedAtMu := sync.Mutex{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo":
			updatedAtMu.Lock()
			curr := updatedAt
			updatedAtMu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"updated_at":"%s"}`, curr.Format(time.RFC3339))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	repo := storage.NewRepository()
	require.NoError(t, repo.RegisterChat(1))
	_, err := repo.AddLink(1, "https://github.com/owner/repo", []string{"work"}, nil)
	require.NoError(t, err)

	notify := &notifierMock{}
	svc := tracker.New(
		repo,
		ghclient.New(ts.URL, ts.Client()),
		stackclient.New(ts.URL, ts.Client()),
		notify,
		newTestLogger(),
	)

	// Первый проход: инициализация lastUpdate + отправка уведомления.
	svc.CheckUpdates(context.Background())
	calls := notify.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "https://github.com/owner/repo", calls[0].URL)
	assert.Equal(t, []int64{1}, calls[0].ChatIDs)

	updatedAtMu.Lock()
	updatedAt = updatedAt.Add(5 * time.Minute)
	updatedAtMu.Unlock()

	// Второй проход: timestamp изменился, должно уйти уведомление.
	svc.CheckUpdates(context.Background())
	calls = notify.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "https://github.com/owner/repo", calls[1].URL)
	assert.Equal(t, []int64{1}, calls[1].ChatIDs)

	// Третий проход без изменений: новых уведомлений быть не должно.
	svc.CheckUpdates(context.Background())
	require.Len(t, notify.Calls(), 2)
}
