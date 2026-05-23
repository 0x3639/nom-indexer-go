// Package database owns the Postgres connection pool and the migration
// runner.
//
// NewPool creates a configured *pgxpool.Pool with sensible limits
// (MaxConnLifetime 1h, MaxConnIdleTime 30m, HealthCheckPeriod 1m) and
// pings the connection before returning.
//
// RunMigrations wraps golang-migrate, applying every migration in
// migrations/ on indexer startup. Idempotent — ErrNoChange is ignored.
//
// See docs/operations/deploy.md and docs/migrations/guide.md.
package database
