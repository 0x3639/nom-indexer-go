//go:build integration

// Integration tests for the watchdog goroutine. Gated by the
// `integration` build tag and TEST_DATABASE_URL.
package indexer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		panic("connect: " + err.Error())
	}
	defer pool.Close()

	migrationsPath := os.Getenv("TEST_MIGRATIONS_PATH")
	if migrationsPath == "" {
		wd, _ := os.Getwd()
		migrationsPath = filepath.Join(wd, "..", "..", "migrations")
	}

	db := stdlib.OpenDBFromPool(pool)
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		panic("migrate driver: " + err.Error())
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		panic("migrate instance: " + err.Error())
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic("migrate up: " + err.Error())
	}
	srcErr, dbErr := migrator.Close()
	if srcErr != nil {
		panic("migrate close source: " + srcErr.Error())
	}
	if dbErr != nil {
		panic("migrate close db: " + dbErr.Error())
	}

	testPool = pool
	os.Exit(m.Run())
}

// newTestPool returns the shared pool, truncating only the tables the
// watchdog touches (momentums + indexer_sync_status) so each test runs
// against a clean slate.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL not set; skipping watchdog integration tests")
	}
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `TRUNCATE indexer_sync_status, momentums`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return testPool
}
