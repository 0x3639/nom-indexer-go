---
title: Deploy
---

# Deploy

The canonical deployment is Docker Compose. Two containers: the
indexer binary and Postgres 16. Everything else is configured via env
vars in `.env`.

## Prerequisites

- Docker Engine 24+ with Compose v2.
- Free disk: at least 10 GB for a fresh test-network DB; 50+ GB if
  syncing from genesis on a production chain.
- Outbound WebSocket access to your Zenon node.

## First-time setup

```bash
git clone https://github.com/0x3639/nom-indexer-go.git
cd nom-indexer-go

# Local credentials. .env is gitignored.
cp .env.example .env
# Edit .env and set POSTGRES_PASSWORD (and other overrides if needed).

docker compose up -d
```

The compose file:

- Mounts a Postgres data volume at `./data` (gitignored).
- Wires the indexer to talk to the `postgres` service via the
  `nom-indexer` user network.
- Exposes Postgres on host port 5432 for local introspection.

## Environment variables

See [`config/reference.md`](../config/reference.md) for the full table.
The compose-specific ones are `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB`. The indexer service forwards them as
`DATABASE_USERNAME`, `DATABASE_PASSWORD`, `DATABASE_NAME`.

## Image build

The Dockerfile is multi-stage:

- **Stage 1** — `golang:1.24-alpine` with CGO toolchain (`gcc musl-dev`)
  for secp256k1. `go mod tidy && go build -o /app/indexer ./cmd/indexer`.
- **Stage 2** — `alpine:3.19` runtime with `ca-certificates` + `tzdata`.
  Runs as non-root user `indexer:1000`.

Final image is ~50 MB. `MIGRATIONS_PATH=/app/migrations` is baked into
the runtime.

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
- **Run your own Zenon node.** See
  [`config/node-selection.md`](../config/node-selection.md).
- **Set `BACKFILL_ON_STARTUP=false`** (the default) so restarts don't
  block on gap-filling.
- **Don't commit `.env`.** It contains the DB password.
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
- **Indexer** — there's no HTTP health endpoint yet (planned with the
  API). The current liveness signal is "sync cursor is advancing"; see
  [`monitoring.md`](monitoring.md).
