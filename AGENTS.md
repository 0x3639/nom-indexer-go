# AGENTS.md

Guidance for Codex when working in this repo. The canonical
documentation tree is [`docs/`](docs/) (rendered at
<https://0x3639.github.io/nom-indexer-go/>); this file holds only the
AI-specific bits that don't fit elsewhere.

## Read these first

| Looking for | Read |
|---|---|
| Architecture / goroutines / data flow | `docs/architecture/overview.md` |
| What a table contains and who writes it | `docs/schema/<table>.md` |
| How a contract's events become rows | `docs/indexing/<contract>-contract.md` |
| Env vars / YAML keys / defaults | `docs/config/reference.md` |
| Common dev recipes (add table, add contract, add cron job) | `docs/development/` |
| Migrations how-to + narrative history | `docs/migrations/` |
| Glossary of Zenon-specific terms | `docs/reference/glossary.md` |
| Per-package Go overview | `docs/code-reference/` |

For one-shot ingestion: [`llms-full.txt`](llms-full.txt) concatenates every page.

## AI-specific gotchas

- **`GOWORK=off`** required for builds. The repo has no workspace file, but
  if a parent directory above your checkout introduces one, builds fail.
- **No vendored SDK.** Current `go.mod` uses upstream
  `github.com/0x3639/znn-sdk-go` directly; there is no `vendor-sdk/`
  directory or `replace` directive in this checkout. See
  `docs/reference/known-issues.md` for the historical accelerator-types panic.
- **`reference/`** contains the original Dart `nom_indexer` for comparison.
  Read it; never edit it.
- **`specs/API_SPECIFICATION.md`** is a draft implementation brief, not the
  API reference. The API server has not been built yet. When it is, the
  doc lives in `docs/api/`.
- **No CGO-free builds.** `secp256k1` and `go-zenon` require CGO. Use the
  multi-stage Docker build for production.

## When in doubt, follow these rules

1. **Schema is the contract.** Both the future REST API and the future MCP
   server read these tables directly. Any column change is a public API
   change. Match the migrations/SQL files exactly when editing models or
   repositories.
2. **`safeBigIntToInt64`** is the single overflow path for `*big.Int` →
   `BIGINT`. New code that converts a big.Int MUST go through it, not
   `Int64()` directly. See `internal/indexer/processor.go`.
3. **All amounts/balances/supplies are int64 (BIGINT).** All timestamps are
   Unix seconds (BIGINT). All hashes are 64-char lowercase hex (TEXT).
   These conventions are stable; don't introduce new types without
   updating `docs/schema/conventions.md`.
4. **Per-momentum writes are transactional.** `processMomentum` opens a
   transaction, runs the batch, commits — or rolls back and returns an
   error so the sync loop retries the height. Don't bypass the transaction.

## Common commands

```bash
# Build, test, lint locally.
GOWORK=off go build ./...
GOWORK=off go test ./...
TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./...

# Docker stack.
docker compose up -d --build
docker logs nom-indexer -f
docker compose down

# Dagger CI (lint + test + build).
dagger call ci --source .

# Regenerate docs indexes.
python scripts/docs/gen-llms-txt.py
python scripts/docs/gen-llms-full.py
mkdocs build --strict   # preview the rendered site
```

## Out of scope for this file

- Anything documented in `docs/` (refer there).
- Schema details (refer to `docs/schema/`).
- Operational runbooks (refer to `docs/operations/`).
