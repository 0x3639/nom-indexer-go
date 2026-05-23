---
title: tokens
---

# `tokens`

## Purpose

ZTS token registry — one row per token standard, with current supply,
ownership, decimals, mintability/burnability flags, and rolling counters
that the cron loops keep current.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `token_standard` | `TEXT` | NO | — | Primary key. `zts1…`. |
| `name` | `TEXT` | NO | — | Human name. |
| `symbol` | `TEXT` | NO | — | Ticker. |
| `domain` | `TEXT` | YES | — | Token issuer's domain. |
| `decimals` | `INT` | NO | — | Decimal places (ZNN/QSR = 8). |
| `owner` | `TEXT` | NO | — | Token owner address. |
| `total_supply` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `max_supply` | `BIGINT` | NO | — | int64 cap applies. |
| `is_burnable` | `BOOLEAN` | NO | — | Whether holders can burn. |
| `is_mintable` | `BOOLEAN` | NO | — | Whether owner can mint. |
| `is_utility` | `BOOLEAN` | NO | — | Utility-token flag from the contract. |
| `total_burned` | `BIGINT` | NO | `0` | Cumulative burns (counter, summed from [`token_burns`](token_burns.md)). |
| `last_update_timestamp` | `BIGINT` | NO | `0` | Last `UpdateToken` event timestamp. |
| `holder_count` | `BIGINT` | NO | `0` | Refreshed by the cron loop. |
| `transaction_count` | `BIGINT` | NO | `0` | Incremented per block whose `token_standard` matches. |

## Primary key & indexes

- **Primary key:** `token_standard`.

## Relations

- `token_standard` is referenced by [`balances`](balances.md),
  [`account_blocks`](account_blocks.md),
  [`token_mints`](token_mints.md), [`token_burns`](token_burns.md),
  [`reward_transactions`](reward_transactions.md),
  [`cumulative_rewards`](cumulative_rewards.md),
  [`wrap_token_requests`](wrap_token_requests.md),
  [`unwrap_token_requests`](unwrap_token_requests.md),
  [`bridge_network_tokens`](bridge_network_tokens.md),
  [`bridge_stat_histories`](bridge_stat_histories.md),
  [`token_stat_histories`](token_stat_histories.md).
- `owner` joins to [`accounts.address`](accounts.md).

## Write path

- **`UpsertBatch`** from `processAccountBlocks` whenever the account block
  carries a `TokenInfo` field (token issue or update event), in
  [`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go).
- **`UpdateBurnAmountBatch`** from the Token contract's `Burn` handler in
  [`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).
- **`IncrementTransactionCountBatch`** whenever a block with `TokenInfo`
  is processed.
- **`UpdateLastUpdateTimestampBatch`** from the Token contract's
  `UpdateToken` handler.
- **`UpdateHolderCount`** from the cron loop in
  [`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go)
  (calls `BalanceRepository.GetHolderCount`, refreshes on the
  `cron.token_holders_interval` schedule, default 10m).

## Read patterns

- **Token detail** — direct PK lookup.
- **All tokens** — `SELECT … FROM tokens` (small table; rarely paginates).
- **Cron snapshot** — used by `snapshotTokenStats` to fill
  [`token_stat_histories`](token_stat_histories.md).

## Gotchas

- `holder_count` lags by up to the holder-count interval (default 10 min).
- `total_burned` is the running sum of burn events. `total_supply` is the
  current authoritative value from the most recent token info; the two are
  not strictly related (issue events can mint as well).
- The empty token standard `zts1qqqqqqqqqqqqqqqqtq587y` is **not** a real
  token — it never appears here. See [`account_blocks`](account_blocks.md)
  for where it shows up.
