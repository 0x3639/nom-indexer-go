---
title: pillar_stat_histories
---

# `pillar_stat_histories`

## Purpose

Daily per-pillar snapshot. Captures rank, weight, and active-delegator count
for every pillar on each cron tick (UTC date bucket).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `date` | `DATE` | NO | — | Composite PK. |
| `pillar_owner_address` | `TEXT` | NO | — | Composite PK. |
| `rank` | `INT` | NO | `0` | Carried from `pillars.rank`. |
| `weight` | `BIGINT` | NO | `0` | Carried from `pillars.weight`. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `momentum_rewards` | `BIGINT` | NO | `0` | Reserved (currently always `0` — see Gotchas). |
| `delegate_rewards` | `BIGINT` | NO | `0` | Reserved (currently always `0`). |
| `total_delegators` | `BIGINT` | NO | `0` | From `DelegationRepository.CountActiveByPillar`. |

## Primary key & indexes

- **Primary key:** `(date, pillar_owner_address)`.
- `idx_pillar_stat_histories_owner` on `pillar_owner_address`.

## Relations

- `pillar_owner_address` ↔ [`pillars.owner_address`](pillars.md).
- `total_delegators` derives from active rows in
  [`delegations`](delegations.md).

## Write path

`snapshotPillarStats` in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).
Walks the cached pillar list (`GetPillars`) and writes one row per pillar.

## Read patterns

- **Pillar trend** — `WHERE pillar_owner_address = $1 ORDER BY date`.
- **Today's leaderboard** — `WHERE date = CURRENT_DATE ORDER BY rank`.

## Gotchas

- `momentum_rewards` and `delegate_rewards` are reserved but **not
  populated**. Wiring them in requires joining `reward_transactions`
  through `delegations` history; left as a future enhancement and called
  out in the cron source.
- Revoked pillars still appear here for any date they were active.
- Same upsert semantics as the other stat-history tables.
