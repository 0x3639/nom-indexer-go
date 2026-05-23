---
title: Integration test database
---

# Integration test database

How to provision a Postgres for the `-tags integration` tests.

## Quickest path: piggyback on the compose stack

If you already have `docker compose up -d` running, the Postgres
container is available on `localhost:5432`. Create a separate test
database so test mutations don't touch your real indexer data:

```bash
docker exec nom-indexer-postgres psql -U postgres \
  -c "CREATE DATABASE nom_indexer_test;"
```

Set the env var and run:

```bash
TEST_DATABASE_URL='postgres://postgres:<your-password>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/...
```

## Standalone disposable instance

If you don't want compose running, spin up a one-off Postgres:

```bash
docker run --rm -d \
  --name nom-indexer-test-db \
  -e POSTGRES_PASSWORD=test \
  -p 5432:5432 \
  postgres:16-alpine

TEST_DATABASE_URL='postgres://postgres:test@localhost:5432/postgres?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/...

docker stop nom-indexer-test-db
```

The container removes itself on stop (`--rm`).

## How migrations are applied

`TestMain` in
[`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go)
opens a pgxpool, then runs every migration via `golang-migrate`'s
`Up()`. Migrations are applied once per `go test` invocation,
regardless of how many tests run.

## How tests get a clean slate

`newTestDB(t)` returns the shared pool after running:

```sql
TRUNCATE momentums, accounts, balances, account_blocks, tokens, …
RESTART IDENTITY;
```

Every table is in the truncate list. When you add a new table, update
the truncate list in
[`integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go)
— otherwise stale data leaks between tests.

`RESTART IDENTITY` resets `BIGSERIAL` counters so tests asserting on
specific `id` values stay deterministic.

## Required env

| Var | Required | Default | Description |
|---|---|---|---|
| `TEST_DATABASE_URL` | yes | — | Pgx URL to the test DB. |
| `TEST_MIGRATIONS_PATH` | no | walks up from test file | Override if your test binary runs from an unusual cwd. |

## Common pitfalls

- **Wrong database.** If `TEST_DATABASE_URL` points at your real DB,
  the truncate at the start of each test will wipe production data.
  Always use a dedicated DB name; `nom_indexer_test` is the convention.
- **Port conflict.** Compose binds Postgres on `5432`. Standalone
  containers either need a different port or the compose stack must
  be down.
- **Stale `_test.go` build tag.** New integration test files need
  `//go:build integration` at the top **before** the `package` line.
  Without it, `go test ./...` will fail to compile because of missing
  helpers from the integration-only files.

## Parallel test execution

Integration tests share one DB and rely on `TRUNCATE` for isolation —
that means they can't safely run in parallel inside the same process.
`go test` doesn't enable parallelism by default for in-package tests,
so this works without explicit synchronization. Don't add
`t.Parallel()` to integration tests.

If you genuinely need parallel integration testing, the right move is
multiple separate DBs (e.g., `nom_indexer_test_1`, `_2`, …) and an
env-var-driven dispatcher. Not worth it today.
