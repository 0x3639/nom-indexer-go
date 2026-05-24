# nom-indexer-go

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker&logoColor=white)](https://www.docker.com/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docs](https://img.shields.io/badge/docs-mkdocs--material-526CFE)](https://0x3639.github.io/nom-indexer-go/)

A high-performance blockchain indexer for the [Zenon Network](https://zenon.network), written in Go. Ports the original [Dart nom-indexer](https://github.com/zenon-tools/nom-indexer) to a typed, batched, transactional Postgres pipeline.

The service subscribes to a Zenon node over WebSocket, decodes embedded-contract activity, and writes a normalized 30-table schema. Read-only HTTP API (`cmd/api`, see [`docs/api/`](docs/api/index.md)) and MCP (`cmd/mcp`, see [`docs/mcp/`](docs/mcp/index.md)) services sit in front of those tables and share the same DTO + repository layer.

## Documentation

Full docs live at **[0x3639.github.io/nom-indexer-go](https://0x3639.github.io/nom-indexer-go/)** (or browse [`docs/`](docs/) on GitHub).

- [Architecture](docs/architecture/overview.md) — system context, goroutines, data flow
- [Schema reference](docs/schema/index.md) — every table, write path, gotchas
- [Indexing flow](docs/indexing/index.md) — per-contract event handlers
- [API reference](docs/api/index.md) — HTTP endpoints, auth (HS256 JWT), Swagger UI
- [MCP reference](docs/mcp/index.md) — hosted Model Context Protocol server for AI clients
- [Operations](docs/operations/deploy.md) — deploy, monitor, backfill, runbook
- [Development](docs/development/setup.md) — local setup, recipes
- [Config reference](docs/config/reference.md) — every env var + YAML key
- [Glossary](docs/reference/glossary.md) — Zenon-specific terminology

LLM-friendly flat indexes are at [`llms.txt`](llms.txt) and [`llms-full.txt`](llms-full.txt).

## Quick start

Prerequisites: Docker, Docker Compose. (Optional: a private Zenon node WS URL.)

```bash
git clone https://github.com/0x3639/nom-indexer-go.git
cd nom-indexer-go

# Local credentials (.env is gitignored).
cp .env.example .env
# Edit .env: set POSTGRES_PASSWORD (always required). If you also want to
# run the API or MCP containers, set API_JWT_SECRET — e.g. openssl rand -base64 48.

# Indexer + Postgres only (the default; no read services):
docker compose up -d postgres indexer

# Or include the API and MCP containers (requires API_JWT_SECRET set above,
# unless MCP is isolated with MCP_JWT_SECRET):
docker compose up -d

# Follow the indexer.
docker logs nom-indexer -f

# Check sync progress.
docker exec nom-indexer-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT MAX(height) FROM momentums;"
```

To point at a custom node, set `NODE_URL_WS` in `.env` before `docker compose up`.

For full deploy/operate guidance see [`docs/operations/deploy.md`](docs/operations/deploy.md). For every env var see [`docs/config/reference.md`](docs/config/reference.md).

## What's indexed

A normalized Postgres schema across 30 tables: momentums, account_blocks, balances, accounts (with genesis + flow metrics), tokens, token_mints, token_burns, pillars, pillar_updates, sentinels, stakes, delegations (history), fusions, projects, project_phases, votes, cumulative_rewards, reward_transactions, wrap/unwrap requests, bridge configuration (networks, network tokens, admin, guardians, orchestrator info, security info), and four daily snapshot tables. See [`docs/schema/`](docs/schema/index.md) for the full reference.

## Architecture at a glance

The service coordinates several independent work lanes so momentum processing is not blocked by slow side-tasks:

1. **Sync / subscription** — initial catch-up, then real-time WebSocket subscription with SDK reconnect callbacks.
2. **Bridge sync loop** — every 1 minute: wrap/unwrap requests plus bridge configuration snapshot.
3. **Cached data sync loop** — every 5 minutes: pillars, sentinels, accelerator projects + phases.
4. **Cron loop** — voting activity + token holder counts (10m each), daily stat snapshots (1h).
5. **Backfill** — optional one-shot on startup, plus the `cmd/backfill` tool for ad-hoc gap filling.

Deep dive: [`docs/architecture/overview.md`](docs/architecture/overview.md).

## Development

```bash
# Build locally (CGO required for secp256k1).
GOWORK=off go build ./...

# Unit tests.
GOWORK=off go test ./...

# Integration tests (require a running test Postgres).
TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./...
```

Recipes for adding contract handlers, tables, or cron jobs are in [`docs/development/`](docs/development/setup.md).

## Contributing

Open an issue or pull request. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for PR conventions.

## License

MIT — see [`LICENSE`](LICENSE).
