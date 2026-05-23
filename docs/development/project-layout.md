---
title: Project layout
---

# Project layout

Where everything lives, and what depends on what.

## Directory tree

```
nom-indexer-go/
├── cmd/                       # binaries
│   ├── indexer/                  the main service
│   └── backfill/                 standalone gap-fill tool
├── internal/                  # private packages for this module
│   ├── config/                   Viper-based config + zap logger builder
│   ├── database/                 pgxpool + golang-migrate plumbing
│   ├── models/                   Go structs mirroring the schema + constants
│   ├── repository/               one file per table; CRUD + batch helpers
│   └── indexer/                  the actual indexing logic
│       ├── indexer.go               Run loop, sync, bridge sync, cron orchestration
│       ├── processor.go             processMomentum + processAccountBlocks
│       ├── decoder.go               ABI decoding into TxData
│       ├── embedded.go              per-contract event handlers
│       ├── rewards.go               classifyReward + reward writes
│       ├── cron.go                  voting / holders / daily snapshot loops
│       └── retry.go                 withRetry helper for transient failures
├── migrations/                # 011 numbered up/down SQL files
├── scripts/                   # one-shot ops + dev tools
│   ├── backup.sh, restore.sh     Postgres dump/restore
│   ├── backfill-rewards/         Re-derive reward tables
│   ├── repair-votes/             Re-decode votes
│   ├── phase-outreach/           AZ outreach helper
│   └── docs/                     mkdocs glue (llms.txt generators, tbls runner)
├── docs/                      # mkdocs-material source
├── specs/                     # ADR-style implementation briefs
├── reference/                 # the original Dart nom_indexer (READ ONLY)
├── .dagger/                   # Dagger CI module
├── .github/workflows/         # GitHub Actions
├── data/                      # Postgres volume mount (gitignored)
├── docker-compose.yml         # the deployment unit
├── Dockerfile                 # multi-stage build
├── config.yaml.example        # config template
├── .env.example               # compose env template
├── go.mod / go.sum            # Go modules
├── mkdocs.yml                 # docs site config
└── README.md / CLAUDE.md      # human + AI orientation
```

## Package responsibilities

| Package | Owns | Imports |
|---|---|---|
| `internal/config` | YAML + env config, zap logger construction. | stdlib + viper + zap |
| `internal/database` | pgxpool creation + migration runner. | stdlib + pgx + golang-migrate |
| `internal/models` | Schema-mirror structs, enum types, address constants. | stdlib only |
| `internal/repository` | Per-table CRUD methods + batched variants. | `internal/models` + pgx |
| `internal/indexer` | Sync loop, contract dispatch, ABI decode, cron jobs. | every package above + znn-sdk-go |

## Import direction

```
cmd/* → internal/indexer → internal/repository → internal/models
                       ↘
                        internal/database
                        internal/config
```

- `internal/repository` never imports `internal/indexer`.
- `internal/models` is leaf-most — imports stdlib only (intentional —
  the models become the API contract for the future REST + MCP layers).
- `internal/indexer` is the only package that knows about the SDK.

## Why `internal/`

Every non-leaf package lives under `internal/` so external Go callers
can't import them. The forthcoming API + MCP server will be siblings
of `cmd/indexer/` and they too will live under `internal/api/` /
`internal/mcp/`; nothing outside this module gets to import them.

## What's *not* a package

- **`reference/`** — Dart code, read-only, comparison material.
- **`scripts/`** — `main` programs only; none import each other.
- **`docs/`** — Markdown source for the rendered site.
- **`specs/`** — ADRs / implementation briefs, not code.

## Adding a new package

If you find yourself reaching for a new `internal/` package, ask
whether you can fit the responsibility into an existing one first.
The current shape (config, database, models, repository, indexer)
covers most of the surface; the next plausible additions are
`internal/api/` and `internal/mcp/`.
