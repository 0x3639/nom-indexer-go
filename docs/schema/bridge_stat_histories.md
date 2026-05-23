---
title: bridge_stat_histories
---

# `bridge_stat_histories`

## Purpose

Daily per-(network, chain, token) bridge volume snapshot. Aggregates wrap
and unwrap counts + amounts per UTC date bucket.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `date` | `DATE` | NO | — | Composite PK. |
| `network_class` | `INT` | NO | — | Composite PK. |
| `chain_id` | `INT` | NO | — | Composite PK. |
| `token_standard` | `TEXT` | NO | — | Composite PK. |
| `wrap_tx_count` | `BIGINT` | NO | `0` | Wrap rows that day. |
| `wrapped_amount` | `BIGINT` | NO | `0` | Sum of wrap `amount` that day. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `unwrap_tx_count` | `BIGINT` | NO | `0` | Unwrap rows that day. |
| `unwrapped_amount` | `BIGINT` | NO | `0` | Sum of unwrap `amount` that day. |
| `total_volume` | `BIGINT` | NO | `0` | `wrapped_amount + unwrapped_amount` (snapshot at cron-tick time). |

## Primary key & indexes

- **Primary key:** `(date, network_class, chain_id, token_standard)`.

## Relations

- `(network_class, chain_id)` ↔ [`bridge_networks`](bridge_networks.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- Source data comes from [`wrap_token_requests`](wrap_token_requests.md)
  and [`unwrap_token_requests`](unwrap_token_requests.md), filtered by
  the creation/registration momentum's timestamp.

## Write path

`snapshotBridgeStats` in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).
Two GROUP BY queries against the wrap and unwrap tables (joining momentums
on the day's timestamp range) build the rows; the cron upserts them.

## Read patterns

- **Daily volume for a network/token** — `WHERE network_class = $1 AND
  chain_id = $2 AND token_standard = $3 ORDER BY date`.
- **Top-volume tokens** — `WHERE date = $1 ORDER BY total_volume DESC`.

## Gotchas

- The wrap/unwrap → momentum join is by **height**, so date bucketing
  depends on momentum timestamp (not request creation time, which is
  the same thing for wraps but distinct from claim time for unwraps).
- A row only exists for (network, chain, token) tuples that had activity
  on that date. Don't assume every tuple has a row for every day.
- Same upsert semantics as the other stat-history tables.
