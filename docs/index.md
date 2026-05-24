# nom-indexer-go

A Go service that indexes the Zenon Network blockchain (NoM) into PostgreSQL.
It listens to a Zenon node over WebSocket, decodes embedded-contract activity,
and writes a normalized relational schema. A read-only HTTP API (`cmd/api`,
documented at [API](api/index.md)) reads those tables behind HS256 JWT auth;
the future MCP server will share the same DTO + repository layer.

## Where to start

=== "Operator"

    You run the service and need it to stay healthy.

    1. [Deploy](operations/deploy.md) — `docker compose up` to a running indexer.
    2. [Monitoring](operations/monitoring.md) — sync progress, log fields, alerts.
    3. [Backfill](operations/backfill.md) — fill gaps, repair historical data.
    4. [Runbook](operations/runbook.md) — what to do when things break.

=== "Developer"

    You're adding a contract, a table, or a cron job.

    1. [Setup](development/setup.md) — local Go build, CGO, `GOWORK=off`.
    2. [Project layout](development/project-layout.md) — where things live.
    3. Recipes: [add a contract handler](development/add-contract-handler.md),
       [add a table](development/add-table.md),
       [add a cron job](development/add-cron-job.md).

=== "API / MCP consumer"

    You query the database that this indexer fills.

    1. [API overview](api/index.md) — quick start, auth, conventions, Swagger UI.
    2. [Endpoint catalog](api/endpoints/index.md) — per-domain curl examples.
    3. [Schema overview](schema/index.md) — table-by-table reference for
       direct SQL access or future MCP consumers.
    4. [Schema conventions](schema/conventions.md) — int64 cap, timestamp,
       hash encoding rules that every table follows.
    5. [Glossary](reference/glossary.md) — Zenon-specific terms.
    6. [MCP (forthcoming)](mcp/index.md)

## What's in the docs

| Section | Covers |
|---|---|
| [Architecture](architecture/overview.md) | System context, goroutine layout, data flow, sync recovery. |
| [Schema](schema/index.md) | Every table: columns, write path, read patterns, gotchas. |
| [Indexing](indexing/index.md) | Per-contract event handlers, ABI decoding, reward classification. |
| [Operations](operations/deploy.md) | Deploy, monitor, backfill, backup, failure modes, runbook, scaling. |
| [Development](development/setup.md) | Local setup, recipes for common changes, coding style, CI. |
| [Testing](testing/strategy.md) | Unit vs integration split, test DB, writing tests. |
| [Config](config/reference.md) | Every env var and YAML key. |
| [Migrations](migrations/guide.md) | Writing migrations, rollback discipline, narrative history. |
| [Code reference](code-reference/index.md) | Go package summaries pulled from `doc.go`. |
| [Reference](reference/glossary.md) | Glossary, addresses, FAQ, known issues. |
| [API](api/index.md) | HTTP endpoints, HS256 JWT auth, Swagger UI, per-domain pages. |
| [MCP](mcp/index.md) | (Stub — fills in when the MCP server ships.) |

## Documentation conventions

Every page in `docs/` follows two consistent templates so humans, LLMs, and
tooling can parse them mechanically.

**Per-table schema pages** in `docs/schema/`:

1. Purpose · 2. Columns · 3. Primary key & indexes · 4. Relations
· 5. Write path · 6. Read patterns · 7. Gotchas.

**Per-contract indexing pages** in `docs/indexing/`:

1. Contract address · 2. Methods observed · 3. Per-method write effects
· 4. Special computation · 5. Tests.

Repeated caveats (the int64 cap, the timestamp convention, hash encoding) live
once in `docs/schema/fragments/` and are included where needed.

Code references use repo-relative paths — `internal/indexer/embedded.go` —
so they travel with the repository and resolve against any checkout.

## For LLMs

Two flat indexes live at the repo root:

- [`llms.txt`](https://github.com/0x3639/nom-indexer-go/blob/main/llms.txt) —
  hash-style index with one-line summaries.
- [`llms-full.txt`](https://github.com/0x3639/nom-indexer-go/blob/main/llms-full.txt) —
  every Markdown page concatenated for one-shot ingestion.

The docs workflows regenerate them while building the published site;
contributors should also run the generators locally and commit the
updated files when docs change.
