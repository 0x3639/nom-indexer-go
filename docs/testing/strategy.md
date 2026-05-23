---
title: Testing strategy
---

# Testing strategy

Two tiers: fast unit tests run by default; integration tests gated by
a build tag and a live database.

## Tiers

| Tier | Build tag | Needs DB? | Where |
|---|---|---|---|
| Unit | (none) | No | Co-located `_test.go` files. |
| Integration | `//go:build integration` | Yes — a fresh Postgres | `internal/repository/integration*_test.go` |

Run:

```bash
# Unit only:
GOWORK=off go test ./...

# Unit + integration:
TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./...
```

The build tag means integration tests are invisible to `go build ./...`
and `go test ./...` — they only compile and run when `-tags integration`
is passed.

## What unit tests cover

- Pure helpers — `safeBigIntToInt64`, `formatArg`, `flowColumn`,
  `ParseCronInterval`, `withRetry`, `determineRewardType`,
  `sanitizeJSONForPostgres`.
- ABI decoding via real SDK ABIs (no DB required) —
  [`internal/indexer/decoder_real_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_real_test.go).
- Config parsing + validation — [`internal/config/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/config).
- Cancel-ID / voting-ID roundtrips through the SDK — [`internal/indexer/voting_id_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/voting_id_test.go).

## What integration tests cover

- Per-repository round trips: insert → select → update.
- Composite-key edge cases (`unwrap_token_requests`,
  `bridge_network_tokens`, `delegations`).
- `ON CONFLICT` semantics (idempotency).
- Singleton invariants (`row_id = 1` check on bridge tables).
- Cross-table dependencies via the migration-driven schema.

Each integration test calls `newTestDB(t)` which truncates every table
before returning the shared pool. Migrations run once per process via
`TestMain`.

## What's *not* tested today

- The live RPC path. The indexer assumes the node responds with valid
  data; we don't fake or record RPC interactions.
- The subscription loop. Connection drops and re-subscription are
  exercised manually against a real node; there's no harness.
- The cron loops. The pure-logic helpers are tested; the actual ticker
  + DB writes are exercised by running the indexer.

These gaps are intentional — the cost of mocking them outweighs the
catches.

## Adding a test

Follow the closest existing pattern:

- New pure helper → table-driven test next to the existing tests in
  the same package.
- New repository method → integration test in
  [`integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go) or
  [`integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go).
  Use the `sendBatch` helper (defined in the same file) for batched
  inserts.
- New contract handler → unit test for any pure helper it introduces;
  end-to-end behavior is verified by running the indexer against a
  test node.

See [`writing-tests.md`](writing-tests.md) for concrete patterns and
[`integration-db.md`](integration-db.md) for the test DB setup.

## CI

Dagger's `test` step runs the unit tier today. Integration tests need
a Postgres that the current Dagger setup doesn't provision; the user
expects to run them locally or in a follow-up pipeline.

Adding an integration step to Dagger is plausible — it would spin up a
sidecar Postgres container during the test step. Not done today.

## Coverage

`go test -cover` is the canonical measure. Recent snapshots:

| Package | Coverage |
|---|---|
| `internal/models` | ~100% |
| `internal/repository` | ~32% (integration tests dominate) |
| `internal/config` | ~42% |
| `internal/indexer` | ~10% |

Coverage is not a CI gate — meaningful coverage in repository code
requires real DB integration tests, and pushing that number up purely
for the metric isn't valuable.
