---
title: Coding style
---

# Coding style

What `golangci-lint v2` enforces, plus the unwritten conventions you'll
hit if you don't follow them.

## Linting

Config: [`.golangci.yml`](https://github.com/0x3639/nom-indexer-go/blob/main/.golangci.yml).
Run locally before pushing:

```bash
golangci-lint run ./...
```

CI fails on any warning. The lint surface includes:

- `gofmt` / `gofumpt` formatting.
- `gci` import ordering: stdlib, third-party, local module —
  with `github.com/0x3639/nom-indexer-go/...` as the local group.
- `govet`, `errcheck`, `staticcheck`, `revive`.
- `gocyclo` complexity caps on per-function basis.

## Imports

`gci` is strict. The three groups must be separated by a blank line:

```go
import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5"
    "go.uber.org/zap"

    "github.com/0x3639/nom-indexer-go/internal/models"
    "github.com/0x3639/nom-indexer-go/internal/repository"
)
```

## Logging (zap)

Use structured fields, never `fmt.Sprintf` into a log message:

```go
// Yes:
i.logger.Warn("transient error, retrying",
    zap.String("op", "GetMomentumsByHeight"),
    zap.Int("attempt", n),
    zap.Error(err))

// No:
i.logger.Warn(fmt.Sprintf("transient error op=%s attempt=%d: %v", op, n, err))
```

Standard field names:

| Field | Meaning |
|---|---|
| `height` | Momentum height. |
| `hash` | Any 64-char hex hash. |
| `address` | A Zenon `z1…` address. |
| `op` | Name of the operation a retry is wrapping. |
| `duration` | Wall-clock duration (use `time.Since(start)`). |
| `error` | Always via `zap.Error(err)`. |

## `*big.Int` → `int64`

**Always** go through `safeBigIntToInt64`. It logs a warning and caps
on overflow:

```go
// Yes:
balanceInt64 := safeBigIntToInt64(balanceInfo.Balance, i.logger,
    "balance overflow",
    zap.String("address", addr.String()))

// No:
balanceInt64 := balanceInfo.Balance.Int64()  // silent overflow
```

## Errors

- Wrap with `fmt.Errorf("…: %w", err)` so `errors.Is` and `errors.As`
  work through the chain.
- Don't return `nil` from a function whose error path matters — return
  a meaningful error and let the caller decide.
- Inside `processMomentum`, prefer logging + returning an error so the
  per-momentum transaction rolls back. The sync loop retries on
  failure.

## Batching

Repository methods come in pairs: `Foo` (one-shot) and `FooBatch`
(takes `*pgx.Batch`). The indexer code uses the batch variants
exclusively so all writes for a momentum land in a single transaction.

Don't open a new transaction inside a batch op — it'll deadlock or
corrupt rollback semantics.

## Naming

- Files: `lowercase_with_underscores.go`. Tests are `_test.go`.
- Packages: lowercase, single word.
- Exported types: `CamelCase`. Repository types are `FooRepository`
  with constructor `NewFooRepository(pool)`.
- DB column names in struct tags use `snake_case` matching the schema.

## Comments

- Package-level `doc.go` files where present give a one-paragraph
  overview that ends up in `go doc` and the rendered `docs/code-reference/`.
- Exported types and functions get a `// Foo does …` comment that
  starts with the identifier name.
- **Don't repeat what the code says.** Comments should add WHY, not
  WHAT.

## Testing

- Unit tests are `GOWORK=off go test ./...`.
- Integration tests live behind `//go:build integration` and need
  `TEST_DATABASE_URL` (see
  [`docs/testing/strategy.md`](../testing/strategy.md)).
- Table-driven tests for pure helpers; round-trip tests for repository
  methods.

## Don't

- **Don't `panic()`** inside the indexer hot path. Return an error.
- **Don't open new goroutines** in handlers — long-lived background
  work is managed by `Indexer.Run` (bridge sync, cached-data sync,
  cron) or by the SDK connection lifecycle.
- **Don't bypass `safeBigIntToInt64`**. (Says it again on purpose.)
- **Don't break the batch invariant.** Per-momentum writes are
  one transaction. Splitting them defeats the rollback-on-failure
  guarantee.

## Generated code

`internal/...` is all hand-written. The committed generated artifacts
are the two `llms*.txt` files from `scripts/docs/gen-llms-*.py`.
`docs/schema/_generated.md` is reserved for an optional local `tbls`
appendix, but CI does not generate it today.
