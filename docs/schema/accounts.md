---
title: accounts
---

# `accounts`

## Purpose

One row per address that has ever been observed in a block. Tracks the
account's running block count, delegation state, ZNN/QSR flow metrics,
genesis balance seed, and first/last-activity timestamps.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `address` | `TEXT` | NO | — | Primary key. `z1…`. |
| `block_count` | `BIGINT` | NO | — | Highest per-account block height we've recorded. |
| `public_key` | `TEXT` | YES | — | Hex public key (set the first time we see a send from this address). |
| `delegate` | `TEXT` | NO | `''` | Current pillar owner address; empty when undelegated. |
| `delegation_start_timestamp` | `BIGINT` | NO | `0` | Unix seconds when the current delegation began. |
| `genesis_znn_balance` | `BIGINT` | NO | `0` | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `genesis_qsr_balance` | `BIGINT` | NO | `0` | Same. |
| `znn_sent` | `BIGINT` | NO | `0` | Running ZNN total this account has sent. |
| `znn_received` | `BIGINT` | NO | `0` | Running ZNN total received. |
| `qsr_sent` | `BIGINT` | NO | `0` | Running QSR total sent. |
| `qsr_received` | `BIGINT` | NO | `0` | Running QSR total received. |
| `first_active_at` | `BIGINT` | YES | — | Unix seconds of the earliest block we've observed. NULL = never seen. |
| `last_active_at` | `BIGINT` | YES | — | Unix seconds of the most recent block observed. NULL = never seen. |

## Primary key & indexes

- **Primary key:** `address`.
- `idx_accounts_delegate` on `delegate`.
- `idx_accounts_first_active_at`, `idx_accounts_last_active_at`.

## Relations

- `address` is referenced by [`balances`](balances.md), [`account_blocks`](account_blocks.md)
  (`address` and `to_address`), [`stakes`](stakes.md), [`fusions`](fusions.md),
  [`reward_transactions`](reward_transactions.md), [`cumulative_rewards`](cumulative_rewards.md),
  [`delegations.delegator_address`](delegations.md), and several others.
- `delegate` joins to [`pillars.owner_address`](pillars.md).

## Write path

- **Basic upsert** — `UpsertBatch` from `processAccountBlocks` for every
  block we touch, in
  [`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go).
  `public_key` only overwrites when the new value is non-empty (preserves
  it after the first known send).
- **Flow + activity** — `AddSendBatch` / `AddReceiveBatch` in
  [`internal/repository/account.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account.go)
  increment the relevant `znn_*` / `qsr_*` column and update
  `first_active_at` (MIN) / `last_active_at` (MAX) atomically. Only ZNN and
  QSR token standards write to the flow columns; other tokens update activity
  only.
- **Genesis seed** — at `m.Height == 1`, `SetGenesisBalanceBatch` records
  the genesis ZNN/QSR balance from the receive amount.
- **Delegation** — `UpdateDelegateBatch` from the Pillar `Delegate` /
  `Undelegate` handlers in
  [`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).
  The `delegations` history table receives a parallel row (close prior, open
  new).

## Read patterns

- **Account detail** — direct PK lookup.
- **Pillar's delegators (current)** — `WHERE delegate = '<pillar-owner>'`.
- **Active addresses by day** — index on `last_active_at` with a time-window
  filter; used by the network-stats snapshot.
- **Whale / richlist** — join to `balances` and ORDER BY balance.

## Gotchas

- `delegate` is the **current** delegation only; full history is in
  [`delegations`](delegations.md).
- The flow columns are **monotonic running totals**, not point-in-time
  balances. To get a current balance use [`balances`](balances.md) which
  reflects the node's authoritative value.
- `genesis_*_balance` is only seeded for addresses that received a genesis
  block (i.e., were funded at chain creation). For newly created addresses,
  these columns stay at 0 — that does not mean "lost balance", just "not
  in genesis".
- `first_active_at` / `last_active_at` use `NULL`, not `0`, for "never
  observed". Other timestamp columns in the schema use `0` for "unset"; this
  one is the exception because monotonic MIN/MAX of `NULL` and a real value
  picks the real value.
