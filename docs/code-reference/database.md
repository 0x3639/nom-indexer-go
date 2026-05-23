---
title: internal/database
---

# `internal/database`

Source: [`internal/database/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/database)

## Package overview

Two responsibilities, kept apart:

1. `NewPool` constructs a configured `*pgxpool.Pool` for the rest of
   the app to use.
2. `RunMigrations` wraps `golang-migrate` and applies every migration
   in the `migrations/` directory on startup.

See the [`doc.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/database/doc.go)
package docstring.

## Files

| File | Contents |
|---|---|
| [`database.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/database/database.go) | `NewPool` factory + `HealthCheck` helper. |
| [`migrations.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/database/migrations.go) | `RunMigrations` driver. |

## Pool settings

`NewPool` pins:

- `MaxConns = cfg.PoolSize` (default 10).
- `MinConns = 2`.
- `MaxConnLifetime = 1 hour`.
- `MaxConnIdleTime = 30 minutes`.
- `HealthCheckPeriod = 1 minute`.

Pings the pool before returning to fail fast on bad credentials or
unreachable Postgres.

## Migration runner

`RunMigrations(pool, path, logger)`:

1. Opens a `database/sql` handle from the pgxpool (`stdlib.OpenDBFromPool`).
2. Creates a Postgres driver instance for `golang-migrate`.
3. Calls `migrator.Up()` and ignores `ErrNoChange`.
4. Logs the resulting version + dirty flag.

Idempotent: re-running with no pending migrations is a no-op.

## See also

- [`docs/migrations/guide.md`](../migrations/guide.md) — how to add a
  migration.
- [`docs/migrations/rollback.md`](../migrations/rollback.md) — how to
  recover from a dirty migration.
- [`docs/operations/scaling.md`](../operations/scaling.md) — pool
  tuning notes.
