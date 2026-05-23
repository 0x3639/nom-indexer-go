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
