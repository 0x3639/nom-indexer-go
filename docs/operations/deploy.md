---
title: Deploy
---

# Deploy

The canonical deployment is Docker Compose. The core stack is the
indexer binary plus Postgres 16; the same compose file also defines
read-only API and MCP services. Everything is configured via env vars
in `.env`.

## Prerequisites

- Docker Engine 24+ with Compose v2.
- Free disk: at least 10 GB for a fresh test-network DB; 50+ GB if
  syncing from genesis on a production chain.
- Outbound WebSocket access to your Zenon node, **or** the optional
  bundled local znnd (see
  [`znnd-bootstrap.md`](znnd-bootstrap.md) — opt-in via the
  `local-node` compose profile).

## First-time setup

```bash
git clone https://github.com/0x3639/nom-indexer-go.git
cd nom-indexer-go

# Local credentials. .env is gitignored.
cp .env.example .env
# Edit .env. Required: POSTGRES_PASSWORD.
# Required only if you bring up the api or mcp service: API_JWT_SECRET
# (any strong random string; openssl rand -base64 48). Set
# MCP_JWT_SECRET only if MCP should use separate tokens.

# Indexer + Postgres only (most common — read directly from the DB):
docker compose up -d postgres indexer

# Or the full stack including the read-only HTTP API and MCP containers:
docker compose up -d

# Or include a bundled local znnd node + snapshot bootstrap:
docker compose --profile local-node up -d --build
# (also set NODE_URL_WS=ws://znnd:35998 in .env so the indexer talks to
#  the bundled node instead of the public test node — see znnd-bootstrap.md)
```

The compose file:

- Mounts a Postgres data volume at `./data` (gitignored).
- Wires the indexer to talk to the `postgres` service via the
  `nom-indexer` user network.
- Exposes Postgres on host port 5432 for local introspection.
- Defines an optional `api` service (built from `Dockerfile.api`) that
  exposes the read-only HTTP API on `${API_PORT:-8080}` and
  Prometheus `/metrics` on `${API_METRICS_PORT:-9090}`. The api
  container refuses to start without `API_JWT_SECRET` in `.env`.
- Defines an optional `mcp` service (built from `Dockerfile.mcp`) that
  exposes Streamable HTTP MCP on `${MCP_PORT:-8081}` and Prometheus
  `/metrics` on `${MCP_METRICS_PORT:-9091}`. The mcp container refuses
  to start unless either `API_JWT_SECRET` or `MCP_JWT_SECRET` is set.

## Environment variables

See [`config/reference.md`](../config/reference.md) for the full table.
The compose-specific ones are `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB` (Postgres + indexer + read services), the API settings
(`API_JWT_SECRET`, `API_PORT`, `API_METRICS_PORT`,
`API_CORS_ALLOWED_ORIGINS`, `API_RATE_LIMIT_PER_MINUTE`), and the MCP
settings (`MCP_JWT_SECRET`, `MCP_PORT`, `MCP_METRICS_PORT`,
`MCP_CORS_ALLOWED_ORIGINS`, `MCP_RATE_LIMIT_PER_MINUTE`).

## Image build

The indexer `Dockerfile` is multi-stage:

- **Stage 1** — `golang:1.25-alpine` with CGO toolchain (`gcc musl-dev`)
  for secp256k1. `go mod tidy && go build -o /app/indexer ./cmd/indexer`.
- **Stage 2** — `alpine:3.19` runtime with `ca-certificates` + `tzdata`.
  Runs as non-root user `indexer:1000`.

Final image is ~50 MB. `MIGRATIONS_PATH=/app/migrations` is baked into
the runtime.

`Dockerfile.api` and `Dockerfile.mcp` follow the same two-stage pattern
but build CGO-free read-service binaries (`cmd/api`, `cmd/mcp`, and
`cmd/jwt-issue`).

To rebuild after code changes:

```bash
docker compose up -d --build
```

## Upgrade procedure

1. **Take a backup.** `scripts/backup.sh` writes a compressed dump to
   `./backups/`.
2. `git pull` (or `git fetch && git checkout <tag>`).
3. `docker compose up -d --build`. Compose recreates only the changed
   container — Postgres stays up if only the indexer changed.
4. Watch logs: `docker logs nom-indexer -f`. Migrations run on startup;
   look for `migrations completed` with the expected version.

## Production considerations

- **Persist `./data`** on durable storage. Losing the Postgres volume
  forces a sync from genesis.
- **Run your own Zenon node.** Either point `NODE_URL_WS` at an external
  self-hosted node, or use the bundled `local-node` compose profile
  ([`znnd-bootstrap.md`](znnd-bootstrap.md)). See
  [`config/node-selection.md`](../config/node-selection.md) for the
  tradeoffs.
- **Set `BACKFILL_ON_STARTUP=false`** (the default) so restarts don't
  block on gap-filling.
- **Don't commit `.env`.** It contains the DB password.
- **Expose the API/MCP behind TLS.** Don't publish `8080`/`8081` (or the
  `9090`/`9091` metrics ports) to the internet directly. Use the bundled
  Caddy overlay for automatic HTTPS and keep the rest loopback-only — see
  [`reverse-proxy.md`](reverse-proxy.md).
- **Monitor sync progress.** See
  [`monitoring.md`](monitoring.md).

## Service management without compose

If you'd rather run the indexer as a systemd service against a
self-managed Postgres, build the binary (`GOWORK=off go build -o
/usr/local/bin/nom-indexer ./cmd/indexer`) and provide the env vars
listed in [`config/reference.md`](../config/reference.md). The binary
self-runs migrations at startup, same as the container.

## Health checks

- **Postgres** — the compose file's healthcheck uses `pg_isready`.
- **Indexer** — there's no HTTP health endpoint; the liveness signal is
  "sync cursor is advancing"; see [`monitoring.md`](monitoring.md).
- **API** — `/healthz` and `/readyz` on `${API_PORT:-8080}`.
- **MCP** — `/healthz` and `/readyz` on `${MCP_PORT:-8081}`.
