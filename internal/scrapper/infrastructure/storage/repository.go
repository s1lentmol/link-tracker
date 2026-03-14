package storage

import (
	"sort"
	"sync"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/domain"
)

type resource struct {
	id         int64
	url        string
	lastUpdate time.Time
	chatIDs    map[int64]struct{}
}

type subscription struct {
	resourceID int64
	url        string
	tags       []string
	filters    []string
}

type Repository struct {
	mu             sync.RWMutex
	nextID         int64
	chats          map[int64]struct{}
	resourcesByURL map[string]*resource
	subscriptions  map[int64]map[string]subscription
}

func NewRepository() *Repository {
	return &Repository{
		nextID:         1,
		chats:          make(map[int64]struct{}),
		resourcesByURL: make(map[string]*resource),
		subscriptions:  make(map[int64]map[string]subscription),
	}
}

func (r *Repository) RegisterChat(chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; ok {
		return apperr.ErrChatExists
	}

	r.chats[chatID] = struct{}{}
	if _, ok := r.subscriptions[chatID]; !ok {
		r.subscriptions[chatID] = make(map[string]subscription)
	}
	return nil
}

func (r *Repository) DeleteChat(chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; !ok {
		return apperr.ErrChatNotFound
	}

	for url := range r.subscriptions[chatID] {
		res := r.resourcesByURL[url]
		delete(res.chatIDs, chatID)
		if len(res.chatIDs) == 0 {
			delete(r.resourcesByURL, url)
		}
	}

	delete(r.subscriptions, chatID)
	delete(r.chats, chatID)
	return nil
}

func (r *Repository) AddLink(chatID int64, url string, tags []string, filters []string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}

	if _, ok := r.subscriptions[chatID][url]; ok {
		return nil, apperr.ErrLinkExists
	}

	res, ok := r.resourcesByURL[url]
	if !ok {
		res = &resource{
			id:      r.nextID,
			url:     url,
			chatIDs: make(map[int64]struct{}),
		}
		r.resourcesByURL[url] = res
		r.nextID++
	}
	res.chatIDs[chatID] = struct{}{}

	sub := subscription{
		resourceID: res.id,
		url:        url,
		tags:       append([]string(nil), tags...),
		filters:    append([]string(nil), filters...),
	}
	r.subscriptions[chatID][url] = sub

	return &domain.Subscription{ID: res.id, URL: url, Tags: sub.tags, Filters: sub.filters}, nil
}

func (r *Repository) RemoveLink(chatID int64, url string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}

	sub, ok := r.subscriptions[chatID][url]
	if !ok {
		return nil, apperr.ErrLinkNotFound
	}
	delete(r.subscriptions[chatID], url)

	if res, ok := r.resourcesByURL[url]; ok {
		delete(res.chatIDs, chatID)
		if len(res.chatIDs) == 0 {
			delete(r.resourcesByURL, url)
		}
	}

	return &domain.Subscription{ID: sub.resourceID, URL: sub.url, Tags: sub.tags, Filters: sub.filters}, nil
}

func (r *Repository) ListLinks(chatID int64) ([]domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.chats[chatID]; !ok {
		return nil, apperr.ErrChatNotFound
	}

	chatSubs := r.subscriptions[chatID]
	result := make([]domain.Subscription, 0, len(chatSubs))
	for _, sub := range chatSubs {
		result = append(result, domain.Subscription{
			ID:      sub.resourceID,
			URL:     sub.url,
			Tags:    append([]string(nil), sub.tags...),
			Filters: append([]string(nil), sub.filters...),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].URL < result[j].URL
	})

	return result, nil
}

func (r *Repository) Resources() []domain.Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	resources := make([]domain.Resource, 0, len(r.resourcesByURL))
	for _, res := range r.resourcesByURL {
		chatIDs := make([]int64, 0, len(res.chatIDs))
		for chatID := range res.chatIDs {
			chatIDs = append(chatIDs, chatID)
		}
		sort.Slice(chatIDs, func(i, j int) bool { return chatIDs[i] < chatIDs[j] })

		resources = append(resources, domain.Resource{
			ID:         res.id,
			URL:        res.url,
			LastUpdate: res.lastUpdate,
			ChatIDs:    chatIDs,
		})
	}

	return resources
}

func (r *Repository) SetLastUpdate(url string, ts time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if res, ok := r.resourcesByURL[url]; ok {
		res.lastUpdate = ts
	}
}
