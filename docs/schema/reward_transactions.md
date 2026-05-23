---
title: reward_transactions
---

# `reward_transactions`

## Purpose

One row per reward-receive block: the actual on-chain event that credited
ZNN or QSR to an address from one of the reward sources (Pillar, Sentinel,
Stake, Liquidity contracts). Companion to the rolled-up
[`cumulative_rewards`](cumulative_rewards.md).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `hash` | `TEXT` | NO | — | Primary key. The receive block's hash. |
| `address` | `TEXT` | NO | — | Receiver. |
| `reward_type` | `SMALLINT` | NO | — | `RewardType` enum (see [`cumulative_rewards`](cumulative_rewards.md)). |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `account_height` | `BIGINT` | NO | — | Per-account block height. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `token_standard` | `TEXT` | NO | — | ZNN or QSR (or LP/utility token for liquidity rewards). |
| `source_address` | `TEXT` | NO | — | The embedded reward contract that emitted the reward. |

## Primary key & indexes

- **Primary key:** `hash`.
- `idx_reward_transactions_address`, `idx_reward_transactions_reward_type`,
  `idx_reward_transactions_momentum_height`.

## Relations

- `address` ↔ [`accounts.address`](accounts.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `source_address` is one of the embedded contracts —
  see [`docs/reference/addresses.md`](../reference/addresses.md).
- `hash` is also the same hash as the corresponding row in
  [`account_blocks`](account_blocks.md).

## Write path

`RewardRepository.InsertRewardTransactionBatch` from
[`internal/indexer/rewards.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards.go):

- **`indexLiquidityReward`** — receive paired with a send from
  `LiquidityTreasuryAddress`; classified `RewardTypeLiquidity`.
- **`indexReceivedReward`** — receive paired with a contract-send from
  pillar/sentinel/stake/liquidity to the empty token standard; classified
  by `classifyReward` (pillar vs delegation split via
  `IsWithdrawAddress`).

The insert uses `ON CONFLICT (hash) DO NOTHING` so it's idempotent. The
sibling `UpdateCumulativeRewardsBatch` is still queued by the live
indexer in the same batch, so reprocessing already-committed reward
events outside a rollback can double-count the rollup. The
`scripts/backfill-rewards` one-shot avoids that by checking
`RowsAffected()` before touching cumulative rewards — see
[`docs/schema/conventions.md`](conventions.md#batch-writes-and-idempotency).

## Read patterns

- **Per-address reward stream** — `WHERE address = $1 ORDER BY
  momentum_height DESC`.
- **Total rewarded today** — `WHERE momentum_timestamp >= startTs AND
  momentum_timestamp < endTs GROUP BY reward_type, token_standard`.
- **Reward attribution** — join `source_address` against
  [`docs/reference/addresses.md`](../reference/addresses.md) constants.

## Gotchas

- The historical reward-detection bug (pre-`utils.BlockType*` fix) meant
  no rows existed in this table for a long stretch. Run
  [`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards)
  to populate them from `account_blocks`.
- `source_address` distinguishes pillar/sentinel/stake/liquidity — but
  the pillar/delegation **split** lives in `reward_type`, not
  `source_address`. Both pillar and delegate rewards come from
  `PillarAddress` as their source.
- `RewardTypeUnknown` (0) means classifyReward could not categorize — the
  row exists for audit but downstream consumers usually filter it out.
