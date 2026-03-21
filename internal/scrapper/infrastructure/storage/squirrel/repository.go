package squirrel

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/domain"
)

type Repository struct {
	pool *pgxpool.Pool
	sb   sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *Repository) RegisterChat(ctx context.Context, chatID int64) error {
	query, args, err := r.sb.Insert("chats").Columns("id").Values(chatID).ToSql()
	if err != nil {
		return fmt.Errorf("build register chat query: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
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
	defer tx.Rollback(ctx)

	query, args, err := r.sb.Delete("chats").Where(sq.Eq{"id": chatID}).ToSql()
	if err != nil {
		return fmt.Errorf("build delete chat query: %w", err)
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrChatNotFound
	}

	cleanup, cleanupArgs, err := r.sb.Delete("links").
		Where("NOT EXISTS (SELECT 1 FROM chat_links cl WHERE cl.link_id = links.id)").
		ToSql()
	if err != nil {
		return fmt.Errorf("build cleanup links query: %w", err)
	}
	if _, err := tx.Exec(ctx, cleanup, cleanupArgs...); err != nil {
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
	defer tx.Rollback(ctx)

	exists, err := r.chatExists(ctx, tx, chatID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.ErrChatNotFound
	}

	dupQuery, dupArgs, err := r.sb.
		Select("l.id").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID, "l.url": url}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build duplicate check query: %w", err)
	}
	var duplicated int64
	err = tx.QueryRow(ctx, dupQuery, dupArgs...).Scan(&duplicated)
	if err == nil {
		return nil, apperr.ErrLinkExists
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check duplicate link: %w", err)
	}

	var linkID int64
	upsert := `INSERT INTO links(url) VALUES ($1) ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url RETURNING id`
	if err := tx.QueryRow(ctx, upsert, url).Scan(&linkID); err != nil {
		return nil, fmt.Errorf("upsert link: %w", err)
	}

	insertChatLink, insertChatArgs, err := r.sb.
		Insert("chat_links").
		Columns("chat_id", "link_id").
		Values(chatID, linkID).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert chat_link query: %w", err)
	}
	if _, err := tx.Exec(ctx, insertChatLink, insertChatArgs...); err != nil {
		if isForeignKeyViolation(err) {
			return nil, apperr.ErrChatNotFound
		}
		if isUniqueViolation(err) {
			return nil, apperr.ErrLinkExists
		}
		return nil, fmt.Errorf("insert chat link: %w", err)
	}

	if err := r.replaceTags(ctx, tx, chatID, linkID, tags); err != nil {
		return nil, err
	}
	if err := r.replaceFilters(ctx, tx, chatID, linkID, filters); err != nil {
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
	defer tx.Rollback(ctx)

	exists, err := r.chatExists(ctx, tx, chatID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.ErrChatNotFound
	}

	linkQ, linkArgs, err := r.sb.
		Select("l.id").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID, "l.url": url}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select link query: %w", err)
	}
	var linkID int64
	err = tx.QueryRow(ctx, linkQ, linkArgs...).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link by chat/url: %w", err)
	}

	tags, err := r.getTags(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, err
	}
	filters, err := r.getFilters(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, err
	}

	delChatLinkQ, delChatLinkArgs, err := r.sb.
		Delete("chat_links").
		Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build delete chat_link query: %w", err)
	}
	if _, err := tx.Exec(ctx, delChatLinkQ, delChatLinkArgs...); err != nil {
		return nil, fmt.Errorf("delete chat link: %w", err)
	}

	cleanupQ, cleanupArgs, err := r.sb.
		Delete("links").
		Where(sq.Eq{"id": linkID}).
		Where("NOT EXISTS (SELECT 1 FROM chat_links cl WHERE cl.link_id = links.id)").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build cleanup link query: %w", err)
	}
	if _, err := tx.Exec(ctx, cleanupQ, cleanupArgs...); err != nil {
		return nil, fmt.Errorf("cleanup orphan link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &domain.Subscription{ID: linkID, URL: url, Tags: tags, Filters: filters}, nil
}

func (r *Repository) ListLinks(ctx context.Context, chatID int64) ([]domain.Subscription, error) {
	exists, err := r.chatExists(ctx, r.pool, chatID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.ErrChatNotFound
	}

	query, args, err := r.sb.
		Select("l.id", "l.url").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID}).
		OrderBy("l.url").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list links query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
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

	tagsByID, err := r.listTagsByLinkID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	filtersByID, err := r.listFiltersByLinkID(ctx, chatID)
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

	exists, err := r.chatExists(ctx, r.pool, chatID)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrChatNotFound
	}

	linkQ, linkArgs, err := r.sb.
		Select("l.id").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID, "l.url": url}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build select link query: %w", err)
	}
	var linkID int64
	err = r.pool.QueryRow(ctx, linkQ, linkArgs...).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrLinkNotFound
	}
	if err != nil {
		return fmt.Errorf("get link by chat/url: %w", err)
	}

	insertTagQ, insertTagArgs, err := r.sb.
		Insert("link_tags").
		Columns("chat_id", "link_id", "tag").
		Values(chatID, linkID, tag).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert tag query: %w", err)
	}
	if _, err := r.pool.Exec(ctx, insertTagQ, insertTagArgs...); err != nil {
		if isUniqueViolation(err) {
			return apperr.ErrTagExists
		}
		return fmt.Errorf("insert tag: %w", err)
	}
	return nil
}

func (r *Repository) RemoveTag(ctx context.Context, chatID int64, url string, tag string) error {
	if tag == "" {
		return apperr.ErrInvalidRequest
	}

	exists, err := r.chatExists(ctx, r.pool, chatID)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrChatNotFound
	}

	linkQ, linkArgs, err := r.sb.
		Select("l.id").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID, "l.url": url}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build select link query: %w", err)
	}
	var linkID int64
	err = r.pool.QueryRow(ctx, linkQ, linkArgs...).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrLinkNotFound
	}
	if err != nil {
		return fmt.Errorf("get link by chat/url: %w", err)
	}

	deleteTagQ, deleteTagArgs, err := r.sb.
		Delete("link_tags").
		Where(sq.Eq{"chat_id": chatID, "link_id": linkID, "tag": tag}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete tag query: %w", err)
	}
	result, err := r.pool.Exec(ctx, deleteTagQ, deleteTagArgs...)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperr.ErrTagNotFound
	}
	return nil
}

func (r *Repository) ListTags(ctx context.Context, chatID int64, url string) ([]string, error) {
	exists, err := r.chatExists(ctx, r.pool, chatID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.ErrChatNotFound
	}

	linkQ, linkArgs, err := r.sb.
		Select("l.id").
		From("chat_links cl").
		Join("links l ON l.id = cl.link_id").
		Where(sq.Eq{"cl.chat_id": chatID, "l.url": url}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select link query: %w", err)
	}
	var linkID int64
	err = r.pool.QueryRow(ctx, linkQ, linkArgs...).Scan(&linkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link by chat/url: %w", err)
	}

	listQ, listArgs, err := r.sb.
		Select("tag").
		From("link_tags").
		Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).
		OrderBy("tag").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list tags query: %w", err)
	}
	rows, err := r.pool.Query(ctx, listQ, listArgs...)
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
		res, ok := resourcesByID[linkID]
		if !ok {
			res = &domain.Resource{ID: linkID, URL: linkURL}
			if lastUpdate != nil {
				res.LastUpdate = *lastUpdate
			}
			resourcesByID[linkID] = res
			order = append(order, linkID)
		}
		if chatID != nil {
			res.ChatIDs = append(res.ChatIDs, *chatID)
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
	query, args, err := r.sb.
		Update("links").
		Set("last_update", ts).
		Where(sq.Eq{"id": linkID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build set last update query: %w", err)
	}
	if _, err := r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("set last update: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) chatExists(ctx context.Context, q queryRower, chatID int64) (bool, error) {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1)`, chatID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check chat exists: %w", err)
	}
	return exists, nil
}

func (r *Repository) replaceTags(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64, tags []string) error {
	deleteQ, deleteArgs, err := r.sb.Delete("link_tags").Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).ToSql()
	if err != nil {
		return fmt.Errorf("build delete tags query: %w", err)
	}
	if _, err := tx.Exec(ctx, deleteQ, deleteArgs...); err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}

	for _, tag := range uniqueStrings(tags) {
		insertQ, insertArgs, err := r.sb.Insert("link_tags").Columns("chat_id", "link_id", "tag").Values(chatID, linkID, tag).ToSql()
		if err != nil {
			return fmt.Errorf("build insert tag query: %w", err)
		}
		if _, err := tx.Exec(ctx, insertQ, insertArgs...); err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	return nil
}

func (r *Repository) replaceFilters(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64, filters []string) error {
	deleteQ, deleteArgs, err := r.sb.Delete("link_filters").Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).ToSql()
	if err != nil {
		return fmt.Errorf("build delete filters query: %w", err)
	}
	if _, err := tx.Exec(ctx, deleteQ, deleteArgs...); err != nil {
		return fmt.Errorf("delete filters: %w", err)
	}

	for _, filter := range uniqueStrings(filters) {
		insertQ, insertArgs, err := r.sb.Insert("link_filters").Columns("chat_id", "link_id", "filter").Values(chatID, linkID, filter).ToSql()
		if err != nil {
			return fmt.Errorf("build insert filter query: %w", err)
		}
		if _, err := tx.Exec(ctx, insertQ, insertArgs...); err != nil {
			return fmt.Errorf("insert filter: %w", err)
		}
	}
	return nil
}

func (r *Repository) getTags(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64) ([]string, error) {
	query, args, err := r.sb.Select("tag").From("link_tags").Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).OrderBy("tag").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get tags query: %w", err)
	}
	rows, err := tx.Query(ctx, query, args...)
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

func (r *Repository) getFilters(ctx context.Context, tx pgx.Tx, chatID int64, linkID int64) ([]string, error) {
	query, args, err := r.sb.Select("filter").From("link_filters").Where(sq.Eq{"chat_id": chatID, "link_id": linkID}).OrderBy("filter").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get filters query: %w", err)
	}
	rows, err := tx.Query(ctx, query, args...)
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

func (r *Repository) listTagsByLinkID(ctx context.Context, chatID int64) (map[int64][]string, error) {
	query, args, err := r.sb.Select("link_id", "tag").From("link_tags").Where(sq.Eq{"chat_id": chatID}).OrderBy("link_id", "tag").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list tags query: %w", err)
	}
	rows, err := r.pool.Query(ctx, query, args...)
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

func (r *Repository) listFiltersByLinkID(ctx context.Context, chatID int64) (map[int64][]string, error) {
	query, args, err := r.sb.Select("link_id", "filter").From("link_filters").Where(sq.Eq{"chat_id": chatID}).OrderBy("link_id", "filter").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list filters query: %w", err)
	}
	rows, err := r.pool.Query(ctx, query, args...)
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
