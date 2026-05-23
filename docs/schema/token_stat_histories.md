---
title: token_stat_histories
---

# `token_stat_histories`

## Purpose

Daily per-token snapshot. For each (date, token) the row records that day's
mints and burns plus the carried-over supply, holder count, and transaction
count at snapshot time.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `date` | `DATE` | NO | — | Composite PK. UTC midnight bucket. |
| `token_standard` | `TEXT` | NO | — | Composite PK. |
| `daily_minted` | `BIGINT` | NO | `0` | Sum of `token_mints.amount` that day. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `daily_burned` | `BIGINT` | NO | `0` | Sum of `token_burns.amount` that day. |
| `total_supply` | `BIGINT` | NO | `0` | Carried from `tokens.total_supply` at snapshot time. |
| `total_holders` | `BIGINT` | NO | `0` | Carried from `tokens.holder_count`. |
| `total_transactions` | `BIGINT` | NO | `0` | Carried from `tokens.transaction_count`. |

## Primary key & indexes

- **Primary key:** `(date, token_standard)`.
- `idx_token_stat_histories_token` on `token_standard`.

## Relations

- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `daily_minted` is derived from [`token_mints`](token_mints.md);
  `daily_burned` from [`token_burns`](token_burns.md).

## Write path

`snapshotTokenStats` in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).
For each token in `tokens`, the cron calls
`TokenEventRepository.SumDailyMintsBurns(token, date)` to get the day's
volume and combines it with the live `tokens` snapshot.

## Read patterns

- **Today's view for a token** — `WHERE date = CURRENT_DATE AND
  token_standard = $1`.
- **Token supply history** — `WHERE token_standard = $1 ORDER BY date`.
- **Top tokens by daily mints** — `WHERE date = $1 ORDER BY daily_minted
  DESC LIMIT N`.

## Gotchas

- `total_supply`, `total_holders`, `total_transactions` are point-in-time
  snapshots — they reflect the values at the cron tick, not the
  end-of-day values (unless the day's last tick happened at 23:59 UTC).
- Same upsert semantics as
  [`network_stat_histories`](network_stat_histories.md) — safe within the
  day, manual reruns against past dates clobber.
- See [`docs/schema/conventions.md`](conventions.md#timestamps) for the
  date-bucketing SQL the cron uses.
