---
title: internal/config
---

# `internal/config`

Source: [`internal/config/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/config)

## Package overview

Loads the indexer's runtime configuration. Viper handles YAML +
environment-variable layering; this package adds validation and
constructs the zap logger from the parsed `LoggingConfig`.

See the [`doc.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/config/doc.go)
package docstring.

## Files

| File | Contents |
|---|---|
| [`config.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/config/config.go) | `Config`, `NodeConfig`, `DatabaseConfig`, `LoggingConfig`, `CronConfig` structs. `Load`, `Validate`, `BuildLogger`. |

## Key entry points

| Symbol | Purpose |
|---|---|
| `config.Load()` | Read YAML + env + defaults, validate, return `*Config`. Used by `cmd/indexer/main.go` and `cmd/backfill/main.go`. |
| `Config.Validate()` | Returns a non-nil error if any required field is missing or invalid. Called inside `Load`. |
| `Config.Logging.BuildLogger()` | Constructs a configured `*zap.Logger` from `level` + `format`. |

## Config struct shape

```go
type Config struct {
    Node              NodeConfig
    Database          DatabaseConfig
    Logging           LoggingConfig
    Cron              CronConfig
    BackfillOnStartup bool
}
```

Each sub-struct's fields are mapped via Viper's `mapstructure` tags
to YAML keys, and the loader has explicit `BindEnv` calls for
backwards-compatible env-var names (`NODE_URL_WS`, `DATABASE_ADDRESS`,
etc.).

## See also

- [`docs/config/reference.md`](../config/reference.md) — every YAML
  key + env var with defaults and validation rules.
- [`docs/config/cron-intervals.md`](../config/cron-intervals.md) — the
  cron interval tradeoffs.
- [`docs/config/node-selection.md`](../config/node-selection.md) — WS
  vs HTTP, public vs self-hosted.
