package tracker_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	appstorage "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/tracker"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/domain"
	ghclient "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/github"
	stackclient "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/http/stackoverflow"
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

type memoryRepo struct {
	mu            sync.RWMutex
	nextID        int64
	chats         map[int64]struct{}
	subscriptions map[int64]map[string]domain.Subscription
	resourcesByID map[int64]*domain.Resource
	idByURL       map[string]int64
}

func newMemoryRepo() appstorage.Repository {
	return &memoryRepo{
		nextID:        1,
		chats:         make(map[int64]struct{}),
		subscriptions: make(map[int64]map[string]domain.Subscription),
		resourcesByID: make(map[int64]*domain.Resource),
		idByURL:       make(map[string]int64),
	}
}

func (r *memoryRepo) RegisterChat(_ context.Context, chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.chats[chatID]; ok {
		return apperr.ErrChatExists
	}
	r.chats[chatID] = struct{}{}
	r.subscriptions[chatID] = make(map[string]domain.Subscription)
	return nil
}

func (r *memoryRepo) DeleteChat(_ context.Context, chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.chats[chatID]; !ok {
		return apperr.ErrChatNotFound
	}
	for url := range r.subscriptions[chatID] {
		linkID := r.idByURL[url]
		res := r.resourcesByID[linkID]
		res.ChatIDs = removeChat(res.ChatIDs, chatID)
		if len(res.ChatIDs) == 0 {
			delete(r.resourcesByID, linkID)
			delete(r.idByURL, url)
		}
	}
	delete(r.subscriptions, chatID)
	delete(r.chats, chatID)
	return nil
}

func (r *memoryRepo) AddLink(_ context.Context, chatID int64, rawURL string, tags []string, filters []string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}
	if _, ok := r.subscriptions[chatID][rawURL]; ok {
		return nil, apperr.ErrLinkExists
	}

	linkID, exists := r.idByURL[rawURL]
	if !exists {
		linkID = r.nextID
		r.nextID++
		r.idByURL[rawURL] = linkID
		r.resourcesByID[linkID] = &domain.Resource{ID: linkID, URL: rawURL}
	}

	res := r.resourcesByID[linkID]
	if !containsChat(res.ChatIDs, chatID) {
		res.ChatIDs = append(res.ChatIDs, chatID)
	}

	sub := domain.Subscription{
		ID:      linkID,
		URL:     rawURL,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
	}
	r.subscriptions[chatID][rawURL] = sub
	return &sub, nil
}

func (r *memoryRepo) RemoveLink(_ context.Context, chatID int64, rawURL string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}
	sub, ok := r.subscriptions[chatID][rawURL]
	if !ok {
		return nil, apperr.ErrLinkNotFound
	}
	delete(r.subscriptions[chatID], rawURL)

	linkID := r.idByURL[rawURL]
	if res, ok := r.resourcesByID[linkID]; ok {
		res.ChatIDs = removeChat(res.ChatIDs, chatID)
		if len(res.ChatIDs) == 0 {
			delete(r.resourcesByID, linkID)
			delete(r.idByURL, rawURL)
		}
	}
	return &sub, nil
}

func (r *memoryRepo) ListLinks(_ context.Context, chatID int64) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}
	result := make([]domain.Subscription, 0, len(r.subscriptions[chatID]))
	for _, sub := range r.subscriptions[chatID] {
		result = append(result, sub)
	}
	return result, nil
}

func (r *memoryRepo) AddTag(_ context.Context, chatID int64, rawURL string, tag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.chats[chatID]; !ok {
		return apperr.ErrChatNotFound
	}
	sub, ok := r.subscriptions[chatID][rawURL]
	if !ok {
		return apperr.ErrLinkNotFound
	}
	for _, currentTag := range sub.Tags {
		if currentTag == tag {
			return apperr.ErrTagExists
		}
	}
	sub.Tags = append(sub.Tags, tag)
	r.subscriptions[chatID][rawURL] = sub
	return nil
}

func (r *memoryRepo) RemoveTag(_ context.Context, chatID int64, rawURL string, tag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.chats[chatID]; !ok {
		return apperr.ErrChatNotFound
	}
	sub, ok := r.subscriptions[chatID][rawURL]
	if !ok {
		return apperr.ErrLinkNotFound
	}
	filtered := make([]string, 0, len(sub.Tags))
	found := false
	for _, currentTag := range sub.Tags {
		if currentTag == tag {
			found = true
			continue
		}
		filtered = append(filtered, currentTag)
	}
	if !found {
		return apperr.ErrTagNotFound
	}
	sub.Tags = filtered
	r.subscriptions[chatID][rawURL] = sub
	return nil
}

func (r *memoryRepo) ListTags(_ context.Context, chatID int64, rawURL string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}
	sub, ok := r.subscriptions[chatID][rawURL]
	if !ok {
		return nil, apperr.ErrLinkNotFound
	}
	return append([]string(nil), sub.Tags...), nil
}

func (r *memoryRepo) ListResourcesPage(_ context.Context, limit int, offset int) ([]domain.Resource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		return []domain.Resource{}, nil
	}
	resources := make([]domain.Resource, 0, len(r.resourcesByID))
	for _, res := range r.resourcesByID {
		resources = append(resources, *res)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	if offset >= len(resources) {
		return []domain.Resource{}, nil
	}
	end := offset + limit
	if end > len(resources) {
		end = len(resources)
	}
	return append([]domain.Resource(nil), resources[offset:end]...), nil
}

func (r *memoryRepo) SetLastUpdateByLinkID(_ context.Context, linkID int64, ts time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.resourcesByID[linkID]
	if !ok {
		return apperr.ErrLinkNotFound
	}
	res.LastUpdate = ts
	return nil
}

func containsChat(ids []int64, chatID int64) bool {
	for _, id := range ids {
		if id == chatID {
			return true
		}
	}
	return false
}

func removeChat(ids []int64, chatID int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != chatID {
			result = append(result, id)
		}
	}
	return result
}

func TestService_ValidateURL(t *testing.T) {
	t.Parallel()

	svc := tracker.New(newMemoryRepo(), nil, nil, &notifierMock{}, newTestLogger())

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

	repo := newMemoryRepo()
	require.NoError(t, repo.RegisterChat(context.Background(), 1))
	_, err := repo.AddLink(context.Background(), 1, "https://github.com/owner/repo", []string{"work"}, nil)
	require.NoError(t, err)

	notify := &notifierMock{}
	svc := tracker.New(
		repo,
		ghclient.New(ts.URL, ts.Client()),
		stackclient.New(ts.URL, ts.Client()),
		notify,
		newTestLogger(),
	)

	// Первый проход: только инициализация lastUpdate, без уведомления.
	svc.CheckUpdates(context.Background())
	calls := notify.Calls()
	require.Len(t, calls, 0)

	updatedAtMu.Lock()
	updatedAt = updatedAt.Add(5 * time.Minute)
	updatedAtMu.Unlock()

	// Второй проход: timestamp изменился, должно уйти уведомление.
	svc.CheckUpdates(context.Background())
	calls = notify.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "https://github.com/owner/repo", calls[0].URL)
	assert.Equal(t, []int64{1}, calls[0].ChatIDs)

	// Третий проход без изменений: новых уведомлений быть не должно.
	svc.CheckUpdates(context.Background())
	require.Len(t, notify.Calls(), 1)
}
