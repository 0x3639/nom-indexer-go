package database

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

// RunMigrations runs all pending database migrations
func RunMigrations(pool *pgxpool.Pool, migrationsPath string, logger *zap.Logger) error {
	logger.Info("running database migrations", zap.String("path", migrationsPath))

	// Get underlying *sql.DB from pgxpool
	db := stdlib.OpenDBFromPool(pool)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", upErr)
	}

	version, dirty, versionErr := m.Version()
	if versionErr != nil && !errors.Is(versionErr, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", versionErr)
	}

	if errors.Is(versionErr, migrate.ErrNilVersion) {
		logger.Info("no migrations applied yet")
	} else {
		logger.Info("migrations completed",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty))
	}

	return nil
}
