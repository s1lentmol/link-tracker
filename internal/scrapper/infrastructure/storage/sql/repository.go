//nolint:govet // shadowing of short-lived err variables keeps multi-step SQL flows concise in this repository.
package sql

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) RegisterChat(ctx context.Context, chatID int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO chats(id) VALUES ($1)`, chatID)
	if err == nil {
		return nil
	}

	if isUniqueViolation(err) {
		return apperr.ErrChatExists
	}

	return fmt.Errorf("register chat: %w", err)
}

func (r *Repository) DeleteChat(ctx context.Context, chatID int64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrChatNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM links l WHERE NOT EXISTS (SELECT 1 FROM chat_links cl WHERE cl.link_id = l.id)`); err != nil {
		return fmt.Errorf("cleanup orphan links: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) AddLink(ctx context.Context, chatID int64, url string, tags []string, filters []string) (*domain.Subscription, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if exists, err := chatExists(ctx, tx, chatID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrChatNotFound
	}

	var duplicated int64
	err = tx.QueryRow(
		ctx,
		`SELECT l.id FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 AND l.url = $2`,
		chatID,
		url,
	).Scan(&duplicated)
	if err == nil {
		return nil, apperr.ErrLinkExists
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check duplicate link: %w", err)
	}

	var linkID int64
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO links(url) VALUES ($1) ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url RETURNING id`,
		url,
	).Scan(&linkID); err != nil {
		return nil, fmt.Errorf("upsert link: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO chat_links(chat_id, link_id) VALUES ($1, $2)`, chatID, linkID); err != nil {
		if isForeignKeyViolation(err) {
			return nil, apperr.ErrChatNotFound
		}
		if isUniqueViolation(err) {
			return nil, apperr.ErrLinkExists
		}
		return nil, fmt.Errorf("insert chat link: %w", err)
	}

	if err := replaceTags(ctx, tx, chatID, linkID, tags); err != nil {
		return nil, err
	}
	if err := replaceFilters(ctx, tx, chatID, linkID, filters); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &domain.Subscription{
		ID:      linkID,
		URL:     url,
		Tags:    cloneAndSort(tags),
		Filters: cloneAndSort(filters),
	}, nil
}

func (r *Repository) RemoveLink(ctx context.Context, chatID int64, url string) (*domain.Subscription, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if exists, err := chatExists(ctx, tx, chatID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrChatNotFound
	}

	var linkID int64
	err = tx.QueryRow(
		ctx,
		`SELECT l.id FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 AND l.url = $2`,
		chatID,
		url,
	).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link by chat/url: %w", err)
	}

	tags, err := getTags(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, err
	}
	filters, err := getFilters(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM chat_links WHERE chat_id = $1 AND link_id = $2`, chatID, linkID); err != nil {
		return nil, fmt.Errorf("delete chat link: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM links l WHERE l.id = $1 AND NOT EXISTS (SELECT 1 FROM chat_links cl WHERE cl.link_id = l.id)`, linkID); err != nil {
		return nil, fmt.Errorf("cleanup orphan link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &domain.Subscription{
		ID:      linkID,
		URL:     url,
		Tags:    tags,
		Filters: filters,
	}, nil
}

func (r *Repository) ListLinks(ctx context.Context, chatID int64) ([]domain.Subscription, error) {
	if exists, err := chatExists(ctx, r.pool, chatID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrChatNotFound
	}

	rows, err := r.pool.Query(ctx, `SELECT l.id, l.url FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 ORDER BY l.url`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	subs := make([]domain.Subscription, 0)
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(&sub.ID, &sub.URL); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate links: %w", err)
	}

	tagsByID, err := listTagsByLinkID(ctx, r.pool, chatID)
	if err != nil {
		return nil, err
	}
	filtersByID, err := listFiltersByLinkID(ctx, r.pool, chatID)
	if err != nil {
		return nil, err
	}

	for i := range subs {
		subs[i].Tags = tagsByID[subs[i].ID]
		subs[i].Filters = filtersByID[subs[i].ID]
	}

	return subs, nil
}

func (r *Repository) AddTag(ctx context.Context, chatID int64, url string, tag string) error {
	if tag == "" {
		return apperr.ErrInvalidRequest
	}

	if exists, err := chatExists(ctx, r.pool, chatID); err != nil {
		return err
	} else if !exists {
		return apperr.ErrChatNotFound
	}

	var linkID int64
	err := r.pool.QueryRow(
		ctx,
		`SELECT l.id FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 AND l.url = $2`,
		chatID,
		url,
	).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrLinkNotFound
	}
	if err != nil {
		return fmt.Errorf("get link by chat/url: %w", err)
	}

	_, err = r.pool.Exec(ctx, `INSERT INTO link_tags(chat_id, link_id, tag) VALUES ($1, $2, $3)`, chatID, linkID, tag)
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return apperr.ErrTagExists
	}
	return fmt.Errorf("insert tag: %w", err)
}

func (r *Repository) RemoveTag(ctx context.Context, chatID int64, url string, tag string) error {
	if tag == "" {
		return apperr.ErrInvalidRequest
	}

	if exists, err := chatExists(ctx, r.pool, chatID); err != nil {
		return err
	} else if !exists {
		return apperr.ErrChatNotFound
	}

	var linkID int64
	err := r.pool.QueryRow(
		ctx,
		`SELECT l.id FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 AND l.url = $2`,
		chatID,
		url,
	).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrLinkNotFound
	}
	if err != nil {
		return fmt.Errorf("get link by chat/url: %w", err)
	}

	tagResult, err := r.pool.Exec(ctx, `DELETE FROM link_tags WHERE chat_id = $1 AND link_id = $2 AND tag = $3`, chatID, linkID, tag)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if tagResult.RowsAffected() == 0 {
		return apperr.ErrTagNotFound
	}
	return nil
}

func (r *Repository) ListTags(ctx context.Context, chatID int64, url string) ([]string, error) {
	if exists, err := chatExists(ctx, r.pool, chatID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrChatNotFound
	}

	var linkID int64
	err := r.pool.QueryRow(
		ctx,
		`SELECT l.id FROM chat_links cl JOIN links l ON l.id = cl.link_id WHERE cl.chat_id = $1 AND l.url = $2`,
		chatID,
		url,
	).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link by chat/url: %w", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT tag FROM link_tags WHERE chat_id = $1 AND link_id = $2 ORDER BY tag`, chatID, linkID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var currentTag string
		if err := rows.Scan(&currentTag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, currentTag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, nil
}

func (r *Repository) ListResourcesPage(ctx context.Context, limit int, offset int) ([]domain.Resource, error) {
	rows, err := r.pool.Query(ctx, `
		WITH page AS (
			SELECT id, url, last_update
			FROM links
			ORDER BY id
			LIMIT $1 OFFSET $2
		)
		SELECT p.id, p.url, p.last_update, cl.chat_id
		FROM page p
		LEFT JOIN chat_links cl ON cl.link_id = p.id
		ORDER BY p.id, cl.chat_id
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list resources page: %w", err)
	}
	defer rows.Close()

	resourcesByID := make(map[int64]*domain.Resource)
	order := make([]int64, 0)

	for rows.Next() {
		var (
			linkID     int64
			linkURL    string
			lastUpdate *time.Time
			chatID     *int64
		)
		if err := rows.Scan(&linkID, &linkURL, &lastUpdate, &chatID); err != nil {
			return nil, fmt.Errorf("scan resource row: %w", err)
		}

		resource, ok := resourcesByID[linkID]
		if !ok {
			resource = &domain.Resource{ID: linkID, URL: linkURL}
			if lastUpdate != nil {
				resource.LastUpdate = *lastUpdate
			}
			resourcesByID[linkID] = resource
			order = append(order, linkID)
		}

		if chatID != nil {
			resource.ChatIDs = append(resource.ChatIDs, *chatID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resources: %w", err)
	}

	result := make([]domain.Resource, 0, len(order))
	for _, id := range order {
		res := resourcesByID[id]
		sort.Slice(res.ChatIDs, func(i, j int) bool { return res.ChatIDs[i] < res.ChatIDs[j] })
		result = append(result, *res)
	}

	return result, nil
}

func (r *Repository) SetLastUpdateByLinkID(ctx context.Context, linkID int64, ts time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE links SET last_update = $2 WHERE id = $1`, linkID, ts)
	if err != nil {
		return fmt.Errorf("set last update: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type queryer interface {
	queryRower
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func chatExists(ctx context.Context, q queryRower, chatID int64) (bool, error) {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)`, chatID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check chat exists: %w", err)
	}
	return exists, nil
}

func replaceTags(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64, tags []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM link_tags WHERE chat_id = $1 AND link_id = $2`, chatID, linkID); err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}
	for _, tag := range uniqueStrings(tags) {
		if _, err := tx.Exec(ctx, `INSERT INTO link_tags(chat_id, link_id, tag) VALUES ($1, $2, $3)`, chatID, linkID, tag); err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}
	return nil
}

func replaceFilters(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64, filters []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM link_filters WHERE chat_id = $1 AND link_id = $2`, chatID, linkID); err != nil {
		return fmt.Errorf("delete filters: %w", err)
	}
	for _, filter := range uniqueStrings(filters) {
		if _, err := tx.Exec(ctx, `INSERT INTO link_filters(chat_id, link_id, filter) VALUES ($1, $2, $3)`, chatID, linkID, filter); err != nil {
			return fmt.Errorf("insert filter: %w", err)
		}
	}
	return nil
}

func getTags(ctx context.Context, q queryer, chatID int64, linkID int64) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT tag FROM link_tags WHERE chat_id = $1 AND link_id = $2 ORDER BY tag`, chatID, linkID)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

func getFilters(ctx context.Context, q queryer, chatID int64, linkID int64) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT filter FROM link_filters WHERE chat_id = $1 AND link_id = $2 ORDER BY filter`, chatID, linkID)
	if err != nil {
		return nil, fmt.Errorf("get filters: %w", err)
	}
	defer rows.Close()

	var filters []string
	for rows.Next() {
		var filter string
		if err := rows.Scan(&filter); err != nil {
			return nil, fmt.Errorf("scan filter: %w", err)
		}
		filters = append(filters, filter)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filters: %w", err)
	}

	return filters, nil
}

func listTagsByLinkID(ctx context.Context, q queryer, chatID int64) (map[int64][]string, error) {
	rows, err := q.Query(ctx, `SELECT link_id, tag FROM link_tags WHERE chat_id = $1 ORDER BY link_id, tag`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var linkID int64
		var tag string
		if err := rows.Scan(&linkID, &tag); err != nil {
			return nil, fmt.Errorf("scan list tags: %w", err)
		}
		result[linkID] = append(result[linkID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list tags: %w", err)
	}
	return result, nil
}

func listFiltersByLinkID(ctx context.Context, q queryer, chatID int64) (map[int64][]string, error) {
	rows, err := q.Query(ctx, `SELECT link_id, filter FROM link_filters WHERE chat_id = $1 ORDER BY link_id, filter`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list filters: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]string)
	for rows.Next() {
		var linkID int64
		var filter string
		if err := rows.Scan(&linkID, &filter); err != nil {
			return nil, fmt.Errorf("scan list filters: %w", err)
		}
		result[linkID] = append(result[linkID], filter)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list filters: %w", err)
	}
	return result, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func uniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneAndSort(values []string) []string {
	clone := append([]string(nil), uniqueStrings(values)...)
	sort.Strings(clone)
	return clone
}
