---
title: internal/repository
---

# `internal/repository`

Source: [`internal/repository/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/repository)

## Package overview

The data-access layer. One file per table; each exports a Repository
type with both one-shot CRUD methods (`Foo`, `FooByX`, …) and batched
variants (`FooBatch`) that take a `*pgx.Batch` and queue the SQL
without sending.

The indexer uses batched variants exclusively so every write for a
single momentum lands in one transaction.

See the [`doc.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/doc.go)
package docstring.

## Files

| File | Table | Source |
|---|---|---|
| [`repository.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/repository.go) | — | `Repositories` aggregator + `NewRepositories`. |
| [`momentum.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/momentum.go) | [`momentums`](../schema/momentums.md) | |
| [`account.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account.go) | [`accounts`](../schema/accounts.md) | Plus the `flowColumn` helper. |
| [`account_block.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account_block.go) | [`account_blocks`](../schema/account_blocks.md) | Plus `sanitizeJSONForPostgres`. |
| [`balance.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/balance.go) | [`balances`](../schema/balances.md) | |
| [`token.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/token.go) | [`tokens`](../schema/tokens.md) | |
| [`token_event.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/token_event.go) | [`token_mints`](../schema/token_mints.md), [`token_burns`](../schema/token_burns.md) | |
| [`pillar.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/pillar.go) | [`pillars`](../schema/pillars.md) | Plus `IsWithdrawAddress`. |
| [`pillar_update.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/pillar_update.go) | [`pillar_updates`](../schema/pillar_updates.md) | |
| [`sentinel.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/sentinel.go) | [`sentinels`](../schema/sentinels.md) | |
| [`stake.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/stake.go) | [`stakes`](../schema/stakes.md) | |
| [`delegation.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/delegation.go) | [`delegations`](../schema/delegations.md) | |
| [`fusion.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/fusion.go) | [`fusions`](../schema/fusions.md) | |
| [`project.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/project.go) | [`projects`](../schema/projects.md) | |
| [`project_phase.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/project_phase.go) | [`project_phases`](../schema/project_phases.md) | |
| [`vote.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/vote.go) | [`votes`](../schema/votes.md) | |
| [`reward.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/reward.go) | [`reward_transactions`](../schema/reward_transactions.md), [`cumulative_rewards`](../schema/cumulative_rewards.md) | |
| [`bridge.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/bridge.go) | [`wrap_token_requests`](../schema/wrap_token_requests.md), [`unwrap_token_requests`](../schema/unwrap_token_requests.md) | Plus `GetWrapSyncStopHeight` / `GetUnwrapSyncStopHeight`. |
| [`bridge_config.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/bridge_config.go) | All 6 bridge-config tables. | `MarkGuardiansAbsent` sweep. |
| [`stat_history.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/stat_history.go) | All 4 `_stat_histories` tables. | |

## Conventions

- **One Repository type per table** (sometimes two tables when they're
  tightly coupled, e.g., `reward.go` for both reward tables).
- **`Foo` + `FooBatch` pairs** for every write method. `Foo` opens its
  own connection; `FooBatch` queues into a caller-owned `*pgx.Batch`.
- **`Get…` methods** return `(*models.Foo, error)` for single-row
  fetches; `([]*models.Foo, error)` for multi-row.

## See also

- [`docs/development/add-table.md`](../development/add-table.md) — recipe.
- [`docs/schema/conventions.md`](../schema/conventions.md) — batch / idempotency rules.
