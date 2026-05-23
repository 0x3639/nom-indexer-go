---
title: sentinels
---

# `sentinels`

## Purpose

Active sentinel node registrations. Refreshed from `SentinelApi.GetAllActive`
on the cached-data sync cadence (5 minutes). Sentinels are a lighter-weight
participant class than pillars — they don't produce momentums but earn
sentinel-class rewards.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `owner` | `TEXT` | NO | — | Primary key. Sentinel's controlling address. |
| `registration_timestamp` | `BIGINT` | NO | — | Unix seconds of registration. |
| `is_revocable` | `BOOLEAN` | NO | — | Whether the sentinel can be revoked now. |
| `revoke_cooldown` | `TEXT` | NO | — | Seconds remaining (stringified to match SDK shape). |
| `active` | `BOOLEAN` | NO | — | Whether the sentinel is currently active. |

## Primary key & indexes

- **Primary key:** `owner`.

## Relations

- `owner` ↔ [`accounts.address`](accounts.md).
- Sentinel-source rewards land in [`reward_transactions`](reward_transactions.md)
  / [`cumulative_rewards`](cumulative_rewards.md) classified as
  `RewardTypeSentinel`.

## Write path

- **`UpsertBatch`** from `updateCachedData` in
  [`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go).
  Paginated `GetAllActive` calls populate the full active set every 5 min.
- **`SetInactiveBatch`** from the Sentinel `Revoke` handler in
  [`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).

## Read patterns

- **Active sentinel count** — `SELECT COUNT(*) WHERE active = true`. Used
  by the network-stats snapshot.
- **Sentinel by owner** — PK lookup.

## Gotchas

- `revoke_cooldown` is `TEXT`, not numeric — it mirrors the SDK's
  stringified representation. Convert with `::bigint` in queries if you
  need ordering.
- Only `Revoke` events are indexed for state changes; new registrations
  appear via the 5-min cached-data refresh, not on the block they happened
  in.
