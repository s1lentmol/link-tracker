package storage_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/apperr"
	"gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/application/storage"
	migrateinfra "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/migrate"
	rawsqlrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/rawsql"
	squirrelrepo "gitlab.education.tbank.ru/backend-academy-go-2026/homeworks/link-tracker/internal/scrapper/infrastructure/storage/squirrel"
)

func TestPostgresRepositoriesContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		factory func(pool *pgxpool.Pool) storage.Repository
	}{
		{name: "sql", factory: func(pool *pgxpool.Pool) storage.Repository { return rawsqlrepo.New(pool) }},
		{name: "squirrel", factory: func(pool *pgxpool.Pool) storage.Repository { return squirrelrepo.New(pool) }},
	}

	for _, tt := range tests {
		ctx := context.Background()
		dsn, stop := startPostgresWithMigrations(ctx, t)
		pool := mustConnectPool(ctx, t, dsn)

		repo := tt.factory(pool)

		require.NoError(t, repo.RegisterChat(ctx, 101), tt.name)

		sub, err := repo.AddLink(ctx, 101, "https://github.com/org/repo", []string{"work", "backend"}, []string{"is:open"})
		require.NoError(t, err, tt.name)
		require.NotNil(t, sub, tt.name)

		_, err = repo.AddLink(ctx, 101, "https://github.com/org/repo", nil, nil)
		require.Error(t, err, tt.name)
		require.ErrorIs(t, err, apperr.ErrLinkExists, tt.name)

		list, err := repo.ListLinks(ctx, 101)
		require.NoError(t, err, tt.name)
		require.Len(t, list, 1, tt.name)
		assert.Equal(t, "https://github.com/org/repo", list[0].URL, tt.name)
		assert.ElementsMatch(t, []string{"work", "backend"}, list[0].Tags, tt.name)
		assert.ElementsMatch(t, []string{"is:open"}, list[0].Filters, tt.name)

		require.NoError(t, repo.AddTag(ctx, 101, "https://github.com/org/repo", "urgent"), tt.name)
		tags, err := repo.ListTags(ctx, 101, "https://github.com/org/repo")
		require.NoError(t, err, tt.name)
		assert.ElementsMatch(t, []string{"backend", "work", "urgent"}, tags, tt.name)

		err = repo.AddTag(ctx, 101, "https://github.com/org/repo", "urgent")
		require.Error(t, err, tt.name)
		require.ErrorIs(t, err, apperr.ErrTagExists, tt.name)

		require.NoError(t, repo.RemoveTag(ctx, 101, "https://github.com/org/repo", "urgent"), tt.name)
		tags, err = repo.ListTags(ctx, 101, "https://github.com/org/repo")
		require.NoError(t, err, tt.name)
		assert.ElementsMatch(t, []string{"backend", "work"}, tags, tt.name)

		err = repo.RemoveTag(ctx, 101, "https://github.com/org/repo", "urgent")
		require.Error(t, err, tt.name)
		require.ErrorIs(t, err, apperr.ErrTagNotFound, tt.name)

		_, err = repo.RemoveLink(ctx, 101, "https://github.com/org/repo")
		require.NoError(t, err, tt.name)

		list, err = repo.ListLinks(ctx, 101)
		require.NoError(t, err, tt.name)
		assert.Empty(t, list, tt.name)

		err = repo.DeleteChat(ctx, 101)
		require.NoError(t, err, tt.name)

		pool.Close()
		stop()
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

	retryErr := retryMigrate(dsn, "../../../../migrations", 10, 500*time.Millisecond)
	if retryErr != nil {
		t.Skipf("postgres not ready for migrations: %v", retryErr)
	}

	pool := mustConnectPool(ctx, t, dsn)
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

func startPostgresWithMigrations(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	container, dsn, err := startPostgresContainer(ctx)
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}

	migrationErr := retryMigrate(dsn, "../../../../migrations", 10, 500*time.Millisecond)
	if migrationErr != nil {
		_ = container.Terminate(ctx)
		t.Skipf("postgres not ready for migrations: %v", migrationErr)
	}

	stop := func() {
		_ = container.Terminate(ctx)
	}

	return dsn, stop
}

func startPostgresContainer(ctx context.Context) (container *tcpostgres.PostgresContainer, dsn string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			container = nil
			dsn = ""
			err = fmt.Errorf("testcontainers panic: %v", recovered)
		}
	}()

	container, err = tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("linktracker"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	if err != nil {
		return nil, "", err
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}

	return container, dsn, nil
}

func mustConnectPool(ctx context.Context, t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	return pool
}

func retryMigrate(dsn string, migrationsPath string, attempts int, delay time.Duration) error {
	var lastErr error
	for range attempts {
		upErr := migrateinfra.Up(migrationsPath, dsn)
		if upErr == nil {
			return nil
		}
		lastErr = upErr
		time.Sleep(delay)
	}
	if lastErr == nil {
		return errors.New("unknown migration error")
	}
	return lastErr
}
