package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// Register postgres driver for golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// Register file source driver for golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Up(migrationsPath string, dsn string) error {
	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	upErr := m.Up()
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations up: %w", upErr)
	}

	return nil
}
