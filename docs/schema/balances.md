---
title: balances
---

# `balances`

## Purpose

Current token balance for an (address, token_standard) pair, plus the
timestamp of the last refresh. Sourced from the node's
`GetAccountInfoByAddress` call, not derived from `account_blocks`.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `address` | `TEXT` | NO | — | Part of composite PK. |
| `token_standard` | `TEXT` | NO | — | Part of composite PK. |
| `balance` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Momentum timestamp at which this row was last refreshed. |

## Primary key & indexes

- **Primary key:** `(address, token_standard)`.
- `idx_balances_token_standard` on `token_standard`.
- `idx_balances_balance` on `balance` *with* `WHERE balance > 0` (partial index
  for richlists).

## Relations

- `address` ↔ [`accounts.address`](accounts.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).

## Write path

`updateBalances` in
[`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go)
calls `GetAccountInfoByAddress` for every address touched by the current
momentum (limited to momentums with fewer than
`genesisBalanceUpdateThreshold` transactions — 1000 — for performance).
Each balance returned is upserted with `last_updated_timestamp` set to the
momentum's timestamp.

## Read patterns

- **Balance of one (address, token)** — direct PK lookup.
- **All balances for an address** — `WHERE address = $1`.
- **Richlist for a token** — `WHERE token_standard = $1 AND balance > 0
  ORDER BY balance DESC LIMIT N` (uses the partial index).
- **Holder count** — `SELECT COUNT(*) WHERE token_standard = $1 AND
  balance > 0`, used by the daily token-holders cron.

## Gotchas

- **Stale rows.** This table tracks the *last observed* balance, not the
  *current* balance. Momentums with > 1000 transactions skip balance
  updates entirely (see `genesisBalanceUpdateThreshold` in `processor.go`).
  Check `last_updated_timestamp` to see when the row was last refreshed.
- **Genesis is intentionally skipped.** The genesis momentum had > 1000
  txs, so this table doesn't carry genesis balances. Use
  [`accounts.genesis_znn_balance`](accounts.md) / `genesis_qsr_balance` for
  that.
- The indexer does not explicitly delete or filter zero-balance rows.
  Holder-count queries always use `balance > 0`, so a retained zero row
  does not count as a holder.
- The empty token standard `zts1qqqqqqqqqqqqqqqqtq587y` is not expected
  from `GetAccountInfoByAddress`, but the balance updater does not have
  a special-case filter for it.
