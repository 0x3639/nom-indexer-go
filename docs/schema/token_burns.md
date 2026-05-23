---
title: token_burns
---

# `token_burns`

## Purpose

One row per `Token.Burn` event. The burner, the amount, and the token are
captured per-event. Counterpart to [`token_mints`](token_mints.md); together
they reconstruct supply changes that the rolling
[`tokens.total_burned`](tokens.md) counter could not.

Introduced in migration `007`.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `BIGSERIAL` | NO | — | Primary key. |
| `account_block_hash` | `TEXT` | NO | — | The contract-receive block on the Token contract. Unique. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `token_standard` | `TEXT` | NO | — | The token being burned. |
| `burner` | `TEXT` | NO | — | The address that sent the tokens to the Token contract for burning. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |

## Primary key & indexes

- **Primary key:** `id`.
- `uq_token_burns_block_hash` — unique on `account_block_hash`.
- `idx_token_burns_token_standard`, `idx_token_burns_burner`,
  `idx_token_burns_momentum_height`.

## Relations

- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `burner` ↔ [`accounts.address`](accounts.md).
- `account_block_hash` ↔ [`account_blocks.hash`](account_blocks.md).
- `momentum_height` ↔ [`momentums.height`](momentums.md).

## Write path

`indexTokenContract` (Token contract `Burn` method) in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).
The amount and token come from the paired send block, not the decoded
inputs — the SDK `Burn` ABI has no inputs.

The same handler also calls `Token.UpdateBurnAmountBatch` so the
[`tokens.total_burned`](tokens.md) counter stays current. The two writes
happen in the same batch transaction.

## Read patterns

- **Burns per token** — `WHERE token_standard = $1 ORDER BY momentum_height
  DESC`. Used for token deflation curves.
- **Burns by an address** — `WHERE burner = $1`.
- **Daily burn totals** — used by `snapshotTokenStats` to fill
  `token_stat_histories.daily_burned`.

## Gotchas

- The `burner` is the sender of the value to the Token contract — i.e., the
  account that gave up the tokens. There is no separate "issuer" because
  burn is a one-sided operation (tokens leave the supply).
- Pre-migration-007 burn history is **not** in this table. Re-running the
  backfill is straightforward (the data lives in `account_blocks`); a
  one-shot script analogous to `backfill-rewards` would do it.
- The `tokens.total_burned` counter and `SUM(amount)` from this table should
  match in steady state; a discrepancy means the migration 007 backfill
  hasn't been run (or the counter was last updated under the older code
  path).
