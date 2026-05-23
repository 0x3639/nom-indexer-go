---
title: network_stat_histories
---

# `network_stat_histories`

## Purpose

Daily network-wide snapshot — one row per UTC date. Aggregates total /
daily transaction count, active and total addresses, token / stake /
fusion counts, and current pillar / sentinel populations. Written by the
1-hour cron-loop snapshot job; current-day rows are upserted on each tick
so they stay fresh mid-day.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `date` | `DATE` | NO | — | Primary key. UTC midnight bucket. |
| `total_tx` | `BIGINT` | NO | `0` | Sum of `momentums.tx_count` all-time. |
| `daily_tx` | `BIGINT` | NO | `0` | Sum of `tx_count` for the day. |
| `total_addresses` | `BIGINT` | NO | `0` | Total rows in `accounts`. |
| `daily_addresses` | `BIGINT` | NO | `0` | Accounts whose `first_active_at` is in the day. |
| `active_addresses` | `BIGINT` | NO | `0` | Accounts whose `last_active_at` is in the day. |
| `total_tokens` | `BIGINT` | NO | `0` | Total rows in `tokens`. |
| `daily_tokens` | `BIGINT` | NO | `0` | New tokens that day (always 0 today — see Gotchas). |
| `total_stakes` | `BIGINT` | NO | `0` | Total rows in `stakes`. |
| `daily_stakes` | `BIGINT` | NO | `0` | New stakes that day. |
| `total_fusions` | `BIGINT` | NO | `0` | Total rows in `fusions`. |
| `daily_fusions` | `BIGINT` | NO | `0` | New fusions that day. |
| `total_pillars` | `BIGINT` | NO | `0` | Active pillars (where `is_revoked = false`). |
| `total_sentinels` | `BIGINT` | NO | `0` | Active sentinels. |

## Primary key & indexes

- **Primary key:** `date`.

## Relations

- Aggregations over [`momentums`](momentums.md), [`accounts`](accounts.md),
  [`tokens`](tokens.md), [`stakes`](stakes.md), [`fusions`](fusions.md),
  [`pillars`](pillars.md), [`sentinels`](sentinels.md).

## Write path

`snapshotNetworkStats` in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).
Runs once per cron tick (default 1 hour), upserting the current UTC date's
row. The aggregation query packs all eight counts into a single SELECT
to minimize round-trips.

## Read patterns

- **Most recent snapshot** — `ORDER BY date DESC LIMIT 1`.
- **Trend by day** — `WHERE date BETWEEN $1 AND $2 ORDER BY date`.

## Gotchas

- `daily_tokens` is currently always `0` — the cron query doesn't filter
  `tokens` by a creation timestamp because `tokens` doesn't track one.
  Fixing this requires adding a `created_at` column to `tokens`; not
  implemented today.
- Date comparison uses Postgres `DATE` semantics — be explicit about UTC
  bucketing when querying from a client (`AT TIME ZONE 'UTC'`).
- Upsert is **replace, not accumulate** — re-running the cron mid-day is
  safe; running it after midnight rewrites yesterday with stale numbers
  if the underlying tables haven't moved. Avoid manual reruns against
  past dates.
