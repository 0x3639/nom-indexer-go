---
title: cumulative_rewards
---

# `cumulative_rewards`

## Purpose

Running per-(address, reward type, token) total of reward amounts received.
Updated additively as new reward receives are indexed. The companion
event-keyed table is [`reward_transactions`](reward_transactions.md).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `SERIAL` | NO | — | Primary key. |
| `address` | `TEXT` | NO | — | The reward receiver. |
| `reward_type` | `SMALLINT` | NO | — | `RewardType` enum from [`internal/models/models.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/models.go) — 0=Unknown, 1=Stake, 2=Delegation, 3=Liquidity, 4=Sentinel, 5=Pillar. |
| `amount` | `BIGINT` | NO | — | Running total. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `token_standard` | `TEXT` | NO | — | The token being received. |

## Primary key & indexes

- **Primary key:** `id`.
- `UNIQUE (address, reward_type, token_standard)` — the natural key.

## Relations

- `address` ↔ [`accounts.address`](accounts.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- One row here is the rollup of many rows in
  [`reward_transactions`](reward_transactions.md) with the same key tuple.

## Write path

`RewardRepository.UpdateCumulativeRewardsBatch` (from
[`internal/repository/reward.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/reward.go))
runs an `INSERT … ON CONFLICT (address, reward_type, token_standard) DO
UPDATE SET amount = existing + EXCLUDED`. Called from
`indexReceivedReward` / `indexLiquidityReward` in
[`internal/indexer/rewards.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards.go).

## Read patterns

- **Total rewards by an address** — `WHERE address = $1` (groups by type
  and token).
- **Lifetime ZNN rewards** — `WHERE token_standard = $1 AND reward_type =
  ANY($2)`.
- **Top earners by type** — `WHERE reward_type = $1 ORDER BY amount DESC`.

## Gotchas

- **Additive updates are not idempotent outside the original transaction.**
  Re-running historical processing without first removing the contributing
  reward_transactions rows will double-count. The
  [`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards)
  script uses `ON CONFLICT (hash) DO NOTHING` on the event table first and
  only updates this table on a successful insert, to avoid that.
- See [`docs/schema/conventions.md`](conventions.md#batch-writes-and-idempotency).
- `RewardTypeDelegation` (2) was historically empty until migration 011's
  reward classification fix; rows with type 2 should now reflect delegate
  rewards correctly going forward.
