package storage

import (
	"context"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/domain"
)

type Repository interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*domain.Subscription, error)
	RemoveLink(ctx context.Context, chatID int64, url string) (*domain.Subscription, error)
	ListLinks(ctx context.Context, chatID int64) ([]domain.Subscription, error)
	AddTag(ctx context.Context, chatID int64, url string, tag string) error
	RemoveTag(ctx context.Context, chatID int64, url string, tag string) error
	ListTags(ctx context.Context, chatID int64, url string) ([]string, error)

	ListResourcesPage(ctx context.Context, limit int, offset int) ([]domain.Resource, error)
	SetLastUpdateByLinkID(ctx context.Context, linkID int64, ts time.Time) error
}
