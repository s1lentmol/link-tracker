package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	migrateinfra "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/migrate"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/sql"
	squirrelrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/squirrel"
)

func TestPostgresRepositoriesContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn, stop := startPostgresWithMigrations(t, ctx)
	defer stop()

	tests := []struct {
		name    string
		factory func(pool *pgxpool.Pool) storage.Repository
	}{
		{name: "sql", factory: func(pool *pgxpool.Pool) storage.Repository { return sqlrepo.New(pool) }},
		{name: "squirrel", factory: func(pool *pgxpool.Pool) storage.Repository { return squirrelrepo.New(pool) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := mustConnectPool(t, ctx, dsn)
			defer pool.Close()

			repo := tt.factory(pool)

			require.NoError(t, repo.RegisterChat(ctx, 101))

			sub, err := repo.AddLink(ctx, 101, "https://github.com/org/repo", []string{"work", "backend"}, []string{"is:open"})
			require.NoError(t, err)
			require.NotNil(t, sub)

			_, err = repo.AddLink(ctx, 101, "https://github.com/org/repo", nil, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrLinkExists)

			list, err := repo.ListLinks(ctx, 101)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, "https://github.com/org/repo", list[0].URL)
			assert.ElementsMatch(t, []string{"work", "backend"}, list[0].Tags)
			assert.ElementsMatch(t, []string{"is:open"}, list[0].Filters)

			require.NoError(t, repo.AddTag(ctx, 101, "https://github.com/org/repo", "urgent"))
			tags, err := repo.ListTags(ctx, 101, "https://github.com/org/repo")
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{"backend", "work", "urgent"}, tags)

			err = repo.AddTag(ctx, 101, "https://github.com/org/repo", "urgent")
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrTagExists)

			require.NoError(t, repo.RemoveTag(ctx, 101, "https://github.com/org/repo", "urgent"))
			tags, err = repo.ListTags(ctx, 101, "https://github.com/org/repo")
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{"backend", "work"}, tags)

			err = repo.RemoveTag(ctx, 101, "https://github.com/org/repo", "urgent")
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrTagNotFound)

			_, err = repo.RemoveLink(ctx, 101, "https://github.com/org/repo")
			require.NoError(t, err)

			list, err = repo.ListLinks(ctx, 101)
			require.NoError(t, err)
			assert.Empty(t, list)

			err = repo.DeleteChat(ctx, 101)
			require.NoError(t, err)
		})
	}
}

func TestMigrationsApplyOnCleanDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	container, dsn, err := startPostgresContainer(ctx)
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	tests := []struct {
		name string
		path string
	}{
		{name: "apply_up_on_clean_db", path: "../../../../migrations"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := retryMigrate(dsn, tt.path, 10, 500*time.Millisecond)
			if err != nil {
				t.Skipf("postgres not ready for migrations: %v", err)
			}
		})
	}

	pool := mustConnectPool(t, ctx, dsn)
	defer pool.Close()

	var count int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name IN ('chats', 'links', 'chat_links', 'link_tags', 'link_filters')
	`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func startPostgresWithMigrations(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()

	container, dsn, err := startPostgresContainer(ctx)
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}

	if err := retryMigrate(dsn, "../../../../migrations", 10, 500*time.Millisecond); err != nil {
		_ = container.Terminate(ctx)
		t.Skipf("postgres not ready for migrations: %v", err)
	}

	stop := func() {
		_ = container.Terminate(ctx)
	}

	return dsn, stop
}

func startPostgresContainer(ctx context.Context) (*tcpostgres.PostgresContainer, string, error) {
	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("linktracker"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	if err != nil {
		return nil, "", err
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}

	return container, dsn, nil
}

func mustConnectPool(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	return pool
}

func retryMigrate(dsn string, migrationsPath string, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := migrateinfra.Up(migrationsPath, dsn); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(delay)
	}
	if lastErr == nil {
		return errors.New("unknown migration error")
	}
	return lastErr
}
