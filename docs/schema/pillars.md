---
title: pillars
---

# `pillars`

## Purpose

Current state of every pillar (validator) the network has ever registered.
Refreshed from `PillarApi.GetAll` on the cached-data sync cadence
(5 minutes). Revoked pillars are kept here with `is_revoked = true` and
their historical fields preserved.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `owner_address` | `TEXT` | NO | — | Primary key. Pillar's controller (`z1…`). |
| `producer_address` | `TEXT` | NO | — | Block-production key. |
| `withdraw_address` | `TEXT` | NO | — | Where reward mints land. Used by reward classification. |
| `name` | `TEXT` | NO | — | Human pillar name (unique on the chain). |
| `rank` | `INT` | NO | — | Current rank order. |
| `give_momentum_reward_percentage` | `SMALLINT` | NO | — | Share of momentum rewards kept by pillar (vs. delegators). |
| `give_delegate_reward_percentage` | `SMALLINT` | NO | — | Share of delegate rewards distributed. |
| `is_revocable` | `BOOLEAN` | NO | — | Whether the pillar can currently be revoked. |
| `revoke_cooldown` | `INT` | NO | — | Seconds remaining in cooldown if applicable. |
| `revoke_timestamp` | `BIGINT` | NO | — | Unix seconds when revocation happened, else 0. |
| `weight` | `BIGINT` | NO | — | Voting weight (ZNN staked + delegated). int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `epoch_produced_momentums` | `SMALLINT` | NO | — | Block count in the current epoch. |
| `epoch_expected_momentums` | `SMALLINT` | NO | — | Expected blocks this epoch. |
| `slot_cost_qsr` | `BIGINT` | NO | `0` | QSR burned to spawn this pillar. |
| `spawn_timestamp` | `BIGINT` | NO | `0` | Unix seconds the pillar was registered. |
| `voting_activity` | `REAL` | NO | `0` | Fraction in `[0,1]`: distinct proposals voted on / proposals eligible since spawn. Refreshed by cron. |
| `produced_momentum_count` | `BIGINT` | NO | `0` | All-time block count (incremented per block produced). |
| `is_revoked` | `BOOLEAN` | NO | `false` | Whether the pillar has been revoked. |

## Primary key & indexes

- **Primary key:** `owner_address`.
- `idx_pillars_name`, `idx_pillars_producer_address`, `idx_pillars_withdraw_address`.

## Relations

- `owner_address` ↔ [`pillar_updates.owner_address`](pillar_updates.md) (history),
  [`delegations.pillar_owner_address`](delegations.md), and
  [`pillar_stat_histories.pillar_owner_address`](pillar_stat_histories.md).
- `producer_address` ↔ [`momentums.producer`](momentums.md).
- `withdraw_address` is used by reward classification — see
  [`indexing/rewards.md`](../indexing/rewards.md).

## Write path

- **`UpsertBatch`** from `updateCachedData` in
  [`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
  refreshing the full pillar set every 5 minutes.
- **`UpdateSpawnInfoBatch`** during a Register block when a descendant burn
  carries the slot QSR cost (in `embedded.go`'s pillar handler).
- **`IncrementMomentumCountBatch`** every time a block whose producer is
  this pillar is processed.
- **`UpdateVotingActivity`** from the cron loop in
  [`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).
- **`SetAsRevokedBatch`** from the Pillar `Revoke` handler — sets
  `is_revoked = true` and `revoke_timestamp`, **preserving** other fields
  (producer/withdraw/rank/momentum count). See
  [`docs/reference/known-issues.md`](../reference/known-issues.md) for the
  prior buggy behavior that wiped history.

## Read patterns

- **Active pillars** — `WHERE is_revoked = false ORDER BY rank`.
- **Pillar by name** — `WHERE name = $1`.
- **Pillar by producer** (e.g., enriching `momentums.producer`) —
  `WHERE producer_address = $1`.
- **Withdraw-address lookup** — used by reward classification via
  `IsWithdrawAddress` (checks both `pillars` and `pillar_updates`).

## Gotchas

- Revoked pillars stay in this table — filter on `is_revoked = false` for
  active sets.
- `voting_activity` is refreshed on cron tick (default 10 min); freshly
  spawned pillars start at 0 until the next cron pass.
- `withdraw_address` can change via `UpdatePillar`. The historical
  withdraw addresses are in [`pillar_updates`](pillar_updates.md); the
  current value is here. Reward classification checks both.
- `slot_cost_qsr` is set only on initial Register (not on update). Pre-fix
  rows may have `0` if the descendant-burn lookup failed at insert time.
