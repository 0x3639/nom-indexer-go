---
title: Add a table
---

# Add a table

A new table touches five things: migration, model, repository,
indexing code, schema doc. Each is small individually.

## 1. Migration

Number it next after the highest existing migration. See
[`docs/migrations/guide.md`](../migrations/guide.md) for the file
naming + idempotency rules.

```sql
-- migrations/012_widgets.up.sql
CREATE TABLE IF NOT EXISTS widgets (
    id BIGSERIAL PRIMARY KEY,
    owner_address TEXT NOT NULL,
    amount BIGINT NOT NULL DEFAULT 0,
    momentum_height BIGINT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    created_at_unix BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_widgets_owner ON widgets(owner_address);
CREATE INDEX IF NOT EXISTS idx_widgets_momentum_height ON widgets(momentum_height);
```

```sql
-- migrations/012_widgets.down.sql
DROP TABLE IF EXISTS widgets;
```

## 2. Model

Add a struct in
[`internal/models/models.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/models.go)
mirroring the columns. Use the existing conventions:

- `BIGINT` columns → `int64` field with `db:"…"` tag.
- `TEXT` → `string`.
- Nullable timestamp → `*int64`.

```go
type Widget struct {
    ID                int64  `db:"id"`
    OwnerAddress      string `db:"owner_address"`
    Amount            int64  `db:"amount"`
    MomentumHeight    int64  `db:"momentum_height"`
    MomentumTimestamp int64  `db:"momentum_timestamp"`
    CreatedAtUnix     int64  `db:"created_at_unix"`
}
```

## 3. Repository

Create `internal/repository/widget.go`. Mirror the shape of an
existing one (e.g.,
[`stake.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/stake.go)).
At minimum:

```go
type WidgetRepository struct{ pool *pgxpool.Pool }

func NewWidgetRepository(pool *pgxpool.Pool) *WidgetRepository {
    return &WidgetRepository{pool: pool}
}

func (r *WidgetRepository) Insert(ctx context.Context, w *models.Widget) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO widgets (owner_address, amount, momentum_height,
            momentum_timestamp, created_at_unix)
        VALUES ($1, $2, $3, $4, $5)`,
        w.OwnerAddress, w.Amount, w.MomentumHeight,
        w.MomentumTimestamp, w.CreatedAtUnix)
    return err
}

func (r *WidgetRepository) InsertBatch(batch *pgx.Batch, w *models.Widget) {
    batch.Queue(`
        INSERT INTO widgets (owner_address, amount, momentum_height,
            momentum_timestamp, created_at_unix)
        VALUES ($1, $2, $3, $4, $5)`,
        w.OwnerAddress, w.Amount, w.MomentumHeight,
        w.MomentumTimestamp, w.CreatedAtUnix)
}
```

Wire it into the `Repositories` struct in
[`internal/repository/repository.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/repository.go):

```go
type Repositories struct {
    // ... existing fields ...
    Widget *WidgetRepository
}

func NewRepositories(pool *pgxpool.Pool) *Repositories {
    return &Repositories{
        // ... existing init ...
        Widget: NewWidgetRepository(pool),
    }
}
```

## 4. Indexer integration

Wherever the new rows originate — typically inside a contract
handler — call `i.repos.Widget.InsertBatch(batch, &models.Widget{…})`.
See [`add-contract-handler.md`](add-contract-handler.md).

## 5. Schema doc

Create `docs/schema/widgets.md` following the standard template
(Purpose, Columns, PK & indexes, Relations, Write path, Read patterns,
Gotchas). For every BIGINT amount column, include the int64-cap
fragment:

```markdown
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
```

Add the page to `docs/schema/index.md`'s domain index and to
`mkdocs.yml`'s nav under the relevant domain group.

## 6. Integration test

Add at least one test in
[`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go)
covering the round trip and any non-trivial query patterns.

Also update the `TRUNCATE` list in `newTestDB` so the new table is
reset between tests.

## 7. Verify

```bash
GOWORK=off go build ./...
GOWORK=off go test ./...

TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/...

.venv-docs/bin/python scripts/docs/gen-llms-txt.py
.venv-docs/bin/python scripts/docs/gen-llms-full.py
.venv-docs/bin/mkdocs build --strict
```

Run the applicable checks before opening a PR. CI runs the unit, lint,
container-build, and docs checks; integration tests still need the
local `TEST_DATABASE_URL` flow above.
