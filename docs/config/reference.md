---
title: Configuration reference
---

# Configuration reference

Every knob exposed to operators. Sources are resolved in this order
(later wins):

1. Hard-coded defaults in
   [`internal/config/config.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/config/config.go).
2. `config.yaml` at the repository root (or `/app/config.yaml` inside
   the container).
3. Environment variables.

The Viper-based loader recognizes underscore-separated env vars that
map to YAML dot-paths (`node.ws_url` ↔ `NODE_WS_URL`), plus the
explicit overrides listed below.

## Required

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `database.password` | string | `DATABASE_PASSWORD` (or `POSTGRES_PASSWORD` in compose) | — | Postgres password. Validation fails if empty. |
| `api.jwt_secret` | string | `API_JWT_SECRET` | — | HS256 signing secret. **Required when `cmd/api` runs** and used by `cmd/mcp` unless `mcp.jwt_secret` is set. The indexer ignores this field. Generate with `openssl rand -base64 48`. |
| `mcp.jwt_secret` | string | `MCP_JWT_SECRET` | — | Optional HS256 signing secret for `cmd/mcp`. Required only when `cmd/mcp` runs without `api.jwt_secret`; set it to isolate MCP tokens from REST API tokens. |

## Node

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `node.ws_url` | string | `NODE_URL_WS` | `wss://test.hc1node.com` | WebSocket URL of the Zenon node. |

## Database

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `database.host` | string | `DATABASE_ADDRESS` | `localhost` | |
| `database.port` | int | `DATABASE_PORT` | `5432` | 1–65535. |
| `database.name` | string | `DATABASE_NAME` | `nom_indexer` | |
| `database.user` | string | `DATABASE_USERNAME` | `postgres` | |
| `database.password` | string | `DATABASE_PASSWORD` | — | **Required.** |
| `database.pool_size` | int | (no env var) | `10` | Max pgxpool size. `MinConns` is hardcoded to 2. |

The connection pool also pins `MaxConnLifetime = 1h`, `MaxConnIdleTime = 30m`,
`HealthCheckPeriod = 1m` (see
[`internal/database/database.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/database/database.go)).

## Logging

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `logging.level` | string | `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Case-insensitive. |
| `logging.format` | string | `LOG_FORMAT` | `console` | `console` or `json`. Console uses ISO8601 timestamps and colorized capital levels. |

## Cron

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `cron.voting_activity_interval` | duration | (no env var) | `10m` | How often to refresh `pillars.voting_activity`. Go duration string. |
| `cron.token_holders_interval` | duration | (no env var) | `10m` | How often to refresh `tokens.holder_count`. |

Several other intervals are hardcoded — they're not tunable today:

- Bridge sync loop: `1 minute`
- Cached data sync loop (pillars, sentinels, projects): `5 minutes`
- Stat snapshot loop (network / token / pillar / bridge): `1 hour`

See [`cron-intervals.md`](cron-intervals.md) for the tradeoffs of
each.

## Behavior flags

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `backfill_on_startup` | bool | `BACKFILL_ON_STARTUP` | `false` | If true, fill gaps in `momentums` / `account_blocks` before live sync. Adds startup time proportional to the gap size. |

## API (`cmd/api` only)

The fields below are read only by the `cmd/api` HTTP API binary; the
indexer binary ignores them. See the [API reference](../api/index.md)
for the on-wire contract.

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `api.port` | int | `API_PORT` | `8080` | Public listener port. The docker-compose api service publishes this 1:1 to the host. |
| `api.metrics_port` | int | `API_METRICS_PORT` | `9090` | Separate listener for Prometheus `/metrics`. Bound to `0.0.0.0`; scope to a private network in production. |
| `api.jwt_secret` | string | `API_JWT_SECRET` | — | **Required for `cmd/api`.** HS256 signing secret. See [Required](#required) above. |
| `api.cors_allowed_origins` | string | `API_CORS_ALLOWED_ORIGINS` | `""` (deny) | Comma-separated origin allowlist. Empty disables CORS entirely (browsers will block cross-origin requests). |
| `api.rate_limit_per_minute` | int | `API_RATE_LIMIT_PER_MINUTE` | `60` | Per-JWT-subject sliding-window limit. `0` or negative disables rate limiting. |

## MCP (`cmd/mcp` only)

The fields below are read only by the hosted MCP server binary; the
indexer binary ignores them. See the [MCP reference](../mcp/index.md)
for the on-wire contract and client setup guides.

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `mcp.port` | int | `MCP_PORT` | `8081` | Public listener port for Streamable HTTP at `/mcp`. The docker-compose mcp service publishes this 1:1 to the host. |
| `mcp.metrics_port` | int | `MCP_METRICS_PORT` | `9091` | Separate listener for Prometheus `/metrics`. Bound to `0.0.0.0`; scope to a private network in production. |
| `mcp.jwt_secret` | string | `MCP_JWT_SECRET` | `""` | Optional HS256 signing secret. Empty means `cmd/mcp` falls back to `api.jwt_secret` / `API_JWT_SECRET`; set it only when REST and MCP should have separate tokens. |
| `mcp.cors_allowed_origins` | string | `MCP_CORS_ALLOWED_ORIGINS` | `""` (deny) | Comma-separated origin allowlist for browser MCP clients. Empty disables CORS entirely. |
| `mcp.rate_limit_per_minute` | int | `MCP_RATE_LIMIT_PER_MINUTE` | `60` | Per-JWT-subject sliding-window limit for MCP transport requests. `0` or negative disables rate limiting. |

## Webhooks (`cmd/indexer` only)

The fields below are read only by the indexer binary; the API and MCP
processes ignore them. Disabled by default. The endpoint list, secrets,
and per-endpoint event filters are **YAML-only** — only the master
on/off switch has an env var. See
[`operations/webhooks.md`](../operations/webhooks.md) for event payloads,
the signature scheme, and delivery semantics.

| Field | Type | Env var | Default | Description |
|---|---|---|---|---|
| `webhooks.enabled` | bool | `WEBHOOKS_ENABLED` | `false` | Master switch. When false the dispatcher is never started and event emission allocates nothing. |
| `webhooks.timeout_seconds` | int | (no env var) | `5` | Per-request HTTP timeout, in seconds, applied to each delivery attempt. |
| `webhooks.max_retries` | int | (no env var) | `3` | Resend attempts after the first failure (network error or non-2xx), with linear backoff. Then the event is dropped with a log line. |
| `webhooks.endpoints` | list | (no env var) | `[]` | Subscribers. Each entry has the fields below. An empty list means nothing is delivered even when `enabled` is true. |
| `webhooks.endpoints[].url` | string | (no env var) | — | Destination URL. Each event is `POST`ed as a JSON body. |
| `webhooks.endpoints[].secret` | string | (no env var) | `""` | If set, signs the request with header `X-Webhook-Signature: <hex HMAC-SHA256 of the raw body>`. Empty means unsigned. Stored in plaintext — keep `config.yaml` private. |
| `webhooks.endpoints[].events` | list | (no env var) | `[]` | Allowlist of event types this endpoint receives (`momentum.inserted`, `account_block.inserted`). **Empty or omitted = all events.** |

## Migrations

| Variable | Default | Description |
|---|---|---|
| `MIGRATIONS_PATH` (env only) | `migrations` | Path the migrator searches for `*.{up,down}.sql`. The container sets it to `/app/migrations`. |

## Postgres compose envelope

`docker-compose.yml` reads three additional variables from `.env` and
forwards them to the Postgres container:

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_USER` | `postgres` | Owner role inside the container. |
| `POSTGRES_PASSWORD` | — | **Required.** Same value the indexer's `DATABASE_PASSWORD` should use. |
| `POSTGRES_DB` | `nom_indexer` | Initial database name. |

## Local znnd (compose `local-node` profile)

These are read only when you opt into the bundled local znnd via
`docker compose --profile local-node up -d`. Without the profile the
`znnd` and `znnd-bootstrap` services are not created and these vars are
ignored. See [`operations/znnd-bootstrap.md`](../operations/znnd-bootstrap.md)
for the full walkthrough.

| Variable | Default | Description |
|---|---|---|
| `ZNND_GIT_REF` | `master` | go-zenon git tag/branch/commit the `znnd` image builds from. Pin to a release tag for reproducible builds. |
| `ZNND_BOOTSTRAP_URL` | — | Public snapshot zip URL. The sibling `.hash` URL (same path, `.hash` extension) must contain the sha256. Empty = skip snapshot install and sync znnd from genesis. |
| `ZNND_FORCE_BOOTSTRAP` | `false` | Force the bootstrap to run even if `/data/nom` already exists. Existing chain-data dirs are moved aside with a timestamp suffix (not deleted). Set true for a one-shot re-bootstrap, then unset. |

When the local-node profile is active, set `NODE_URL_WS=ws://znnd:35998`
in `.env` so the indexer container resolves znnd by docker-network
service name instead of dialing the public test node.

The `api` compose service requires `API_JWT_SECRET` in `.env` (the
container refuses to start without it). The `mcp` compose service
requires either `API_JWT_SECRET` or `MCP_JWT_SECRET`; set
`MCP_JWT_SECRET` only when you want isolated MCP key material. Compose
does **not** fail at interpolation time for those optional read
services, so `docker compose up postgres indexer` works without setting
either secret.

## Validation

`Config.Validate` enforces:

- `node.ws_url` non-empty.
- `database.host` non-empty.
- `database.port` in `1..=65535`.
- `database.name` non-empty.
- `database.user` non-empty.
- `database.password` non-empty.

Validation runs at startup; the binary exits non-zero with a clear
message on failure.

## `config.yaml.example`

The repo ships
[`config.yaml.example`](https://github.com/0x3639/nom-indexer-go/blob/main/config.yaml.example)
as the canonical template. Copy to `config.yaml` (gitignored) and
customize.

`.env.example` is the analogous template for compose environment
variables.
