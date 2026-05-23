---
title: momentums
---

# `momentums`

## Purpose

One row per block header. The schema's primary ledger anchor — every other
table that has a `momentum_height` or `momentum_hash` column joins back here.
`MAX(height)` is also the indexer's sync cursor.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `height` | `BIGINT` | NO | — | Primary key. Monotonic non-decreasing from genesis (`1`). |
| `hash` | `TEXT` | NO | — | 64-char hex. See [hash encoding](conventions.md#hash-encoding). |
| `timestamp` | `BIGINT` | NO | — | Unix seconds UTC. See [timestamp caveat](conventions.md#timestamps). |
| `tx_count` | `INT` | NO | — | Number of account blocks in this momentum. |
| `producer` | `TEXT` | NO | — | Pillar producer address (`z1…`). |
| `producer_owner` | `TEXT` | NO | `''` | Owner address of the producing pillar. Resolved at write time via `getPillarInfoForProducer`. |
| `producer_name` | `TEXT` | NO | `''` | Producer pillar's name. Same lookup. |

## Primary key & indexes

- **Primary key:** `height`.
- `idx_momentums_timestamp` on `timestamp` — used by daily-snapshot date bucketing.
- `idx_momentums_producer` on `producer` — used for "blocks produced by pillar" queries.

## Relations

- `account_blocks.momentum_hash` ↔ `momentums.hash`
- `account_blocks.momentum_height` ↔ `momentums.height`
- `wrap_token_requests.creation_momentum_height` ↔ `momentums.height`
- `unwrap_token_requests.registration_momentum_height` ↔ `momentums.height`

## Write path

[`MomentumRepository.InsertBatch`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/momentum.go)
is queued at the end of every `processMomentum` call in
[`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go).
The INSERT uses `ON CONFLICT (height) DO NOTHING`, so a momentum retried after
a partial-batch rollback is idempotent.

The pillar lookup that fills `producer_owner` / `producer_name` runs before
the batch is sent (see `getPillarInfoForProducer`); a brand-new pillar may
appear here with empty fields until the next cached-data refresh.

## Read patterns

- **Sync cursor:** `SELECT MAX(height) FROM momentums` — `GetLatestHeight`.
- **Block detail by height or hash** — equality on the PK or `hash`.
- **Frontier scan** — `WHERE height BETWEEN X AND Y ORDER BY height` for
  pagination.
- **Daily activity** — `WHERE timestamp >= startTs AND timestamp < endTs`,
  used by the network-stats snapshot.

## Gotchas

- The genesis momentum is `height = 1`, not `0`. Sync loops handle this with
  `if dbHeight == 0 { startHeight = 1 } else { startHeight = dbHeight + 1 }`.
- A momentum with `tx_count = 0` is valid — the chain produces blocks even
  when no transactions are pending. Per-block balance updates are skipped
  for those.
- `tx_count` is the count reported by the node, **not** a count of rows in
  `account_blocks`. They can diverge transiently during processing; the
  backfill tool reconciles them via the "incomplete momentum" check
  (`tx_count > 0 AND COALESCE(actual_txs, 0) < tx_count`).
