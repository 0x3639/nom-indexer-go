---
title: Code reference
---

# Code reference

One-paragraph overview per `internal/*` package, with links to the
canonical Go doc on `pkg.go.dev`-style locations (the package's
`doc.go` file) and to the GitHub source.

When `mkdocstrings-go` is wired up in `mkdocs.yml`, this section will
render exported symbols inline. For now it points at the source.

| Package | Overview | Source |
|---|---|---|
| [`internal/config`](config.md) | Viper-driven config loader + zap logger builder. | [`internal/config/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/config) |
| [`internal/database`](database.md) | pgxpool factory + migration runner. | [`internal/database/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/database) |
| [`internal/models`](models.md) | Schema-mirror structs + constants. Leaf of the import graph. | [`internal/models/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/models) |
| [`internal/repository`](repository.md) | Per-table CRUD + batched variants. | [`internal/repository/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/repository) |
| [`internal/indexer`](indexer.md) | Sync + subscription + cron + bridge sync. | [`internal/indexer/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/indexer) |

## Reading order

If you're new to the codebase, read in this order:

1. [`internal/models`](models.md) — the schema-mirror structs are the
   contract everything else assumes.
2. [`internal/repository`](repository.md) — see the access patterns
   used by every write site.
3. [`internal/indexer`](indexer.md) — the actual logic. `processor.go`
   first, then `embedded.go`, then `indexer.go` (sync loop),
   `cron.go`, `rewards.go`.
4. [`internal/database`](database.md) and
   [`internal/config`](config.md) — plumbing.

## See also

- [`docs/development/project-layout.md`](../development/project-layout.md) for
  the package-dependency direction and where things live.
- [`docs/architecture/overview.md`](../architecture/overview.md) for
  the goroutine layout these packages compose into.
