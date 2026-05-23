---
title: Migrations guide
---

# Migrations guide

How to add and ship a schema change.

## File layout

`migrations/NNN_short_name.up.sql` paired with
`migrations/NNN_short_name.down.sql`.

- `NNN` is a zero-padded three-digit sequence (`007`, `008`, etc.).
- `short_name` is `snake_case`, ~3 words.
- Both up and down must exist; the down must reverse the up exactly.

Migrations are driven by
[golang-migrate](https://github.com/golang-migrate/migrate) via
[`internal/database/migrations.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/database/migrations.go).
The indexer runs `Up` on startup; running it twice is safe because
`ErrNoChange` is ignored.

## Authoring rules

1. **Idempotent where PostgreSQL supports it cleanly.** Prefer
   `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, and
   `ALTER TABLE … ADD COLUMN IF NOT EXISTS`. Some older migrations use
   plain `ALTER TABLE`; match the safest form for the operation you're
   adding and verify it against a migrated test DB.
2. **Reversible.** Every down drops/alters what the up created in
   reverse order. If the up adds a column, the down drops it. If the up
   inserts data (rare — usually a backfill belongs in a `scripts/`
   one-shot), the down deletes it.
3. **Atomic, single concern.** One migration changes one thing. Do not
   pack a column add and an unrelated index in the same file.
4. **No data backfills inside migrations.** Use a separate one-shot
   under `scripts/` (e.g.,
   [`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards)).
   This keeps migrations fast and lets ops re-run the backfill out of
   band.
5. **Update the schema docs.** The hand-written `docs/schema/<table>.md`
   page is the source of truth for semantics; update it in the same PR.
   If you maintain the optional `tbls` appendix, regenerate
   `_generated.md` locally with `scripts/docs/gen-schema-tbls.sh`.

## Adding a new table

1. Create the migration files. Example:

    ```sql
    -- migrations/012_widgets.up.sql
    CREATE TABLE IF NOT EXISTS widgets (
        id BIGSERIAL PRIMARY KEY,
        owner_address TEXT NOT NULL,
        amount BIGINT NOT NULL DEFAULT 0,
        created_at BIGINT NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_widgets_owner ON widgets(owner_address);
    ```

    ```sql
    -- migrations/012_widgets.down.sql
    DROP TABLE IF EXISTS widgets;
    ```

2. Add a Go struct in `internal/models/models.go` with `db:"…"` tags.
3. Create `internal/repository/widget.go` mirroring an existing
   repository (e.g., `stake.go`). Add to `Repositories` in
   `repository.go`.
4. Wire writes from the per-contract handler.
5. Add at least one integration test
   ([`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go))
   covering the upsert/select round trip.
6. Add `docs/schema/widgets.md` following the standard template
   (Purpose, Columns, PK & indexes, Relations, Write path, Read
   patterns, Gotchas). See
   [`docs/development/add-table.md`](../development/add-table.md) for
   the full recipe.

## Adding a column to an existing table

```sql
-- migrations/013_accounts_new_field.up.sql
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS new_field BIGINT NOT NULL DEFAULT 0;
```

```sql
-- migrations/013_accounts_new_field.down.sql
ALTER TABLE accounts DROP COLUMN IF EXISTS new_field;
```

Update the `accounts` struct in `models.go` and the SELECT in
`account.go`. Then update `docs/schema/accounts.md`.

## Local testing

```bash
# Apply against a throwaway test DB:
docker exec nom-indexer-postgres psql -U postgres -c "CREATE DATABASE migration_test;"
TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/migration_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/...

# Or run mkdocs to make sure the schema doc renders:
.venv-docs/bin/mkdocs serve
```

The integration test `TestMain` runs all migrations on every test run,
so any new migration is exercised automatically.

## When `golang-migrate` gets stuck

If a migration fails partway through, the `schema_migrations` table
records a "dirty" version. Fix the underlying issue, then:

```bash
# From inside the postgres container:
psql -U postgres -d nom_indexer -c \
  "UPDATE schema_migrations SET dirty = false;"
```

…and re-run. See [`rollback.md`](rollback.md) for the full recovery
playbook.
