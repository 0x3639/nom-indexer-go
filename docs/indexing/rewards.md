---
title: Rewards classification and aggregation
---

# Rewards classification and aggregation

Reward indexing is the most subtle piece of the per-block pipeline. It
has its own subsystem because the BlockType-based detection bug history
made it the most fragile area of the indexer.

## Detection

In `processAccountBlocks`
([`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go)),
after a block is inserted, two checks run:

```go
if block.PairedAccountBlock != nil && block.BlockType == utils.BlockTypeUserReceive {
    if block.PairedAccountBlock.Address == LiquidityTreasuryAddress {
        i.indexLiquidityReward(batch, block, m)
    } else if block.PairedAccountBlock.BlockType == utils.BlockTypeContractSend &&
              block.ToAddress == EmptyAddress &&
              block.TokenStandard == EmptyTokenStandard {
        i.indexReceivedReward(ctx, batch, block, m)
    }
}
```

Translation: the block we're processing must be a `UserReceive` (3),
paired with either:

- a treasury send (liquidity reward), or
- a `ContractSend` (4) **from an embedded reward source**, with the
  receive itself having `to_address = EmptyAddress` and
  `token_standard = EmptyTokenStandard`.

The second clause is what historically broke — see
[`docs/reference/known-issues.md`](../reference/known-issues.md).

## Classification (`classifyReward`)

In [`internal/indexer/rewards.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards.go):

```go
switch sourceAddress {
case SentinelAddress: return RewardTypeSentinel
case StakeAddress:    return RewardTypeStake
case LiquidityAddress:return RewardTypeLiquidity
case PillarAddress:
    if i.repos.Pillar.IsWithdrawAddress(receiverAddress) {
        return RewardTypePillar
    }
    return RewardTypeDelegation
}
```

The pillar/delegation split uses `IsWithdrawAddress`, which checks both
current [`pillars.withdraw_address`](../schema/pillars.md) and historical
[`pillar_updates.withdraw_address`](../schema/pillar_updates.md). If the
receiver matches either set, the reward is classified as
`RewardTypePillar`; otherwise a reward from `PillarAddress` falls through
to `RewardTypeDelegation`.

## Aggregation

Two writes per classified reward, in
[`internal/repository/reward.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/reward.go):

1. **`InsertRewardTransactionBatch`** — `INSERT … ON CONFLICT (hash) DO
   NOTHING` into [`reward_transactions`](../schema/reward_transactions.md).
2. **`UpdateCumulativeRewardsBatch`** — additive upsert into
   [`cumulative_rewards`](../schema/cumulative_rewards.md). In the live
   indexer this is queued unconditionally in the same momentum batch.
   The one-shot backfill script is more defensive: it checks
   `RowsAffected()` on `reward_transactions` and skips the cumulative
   update when the event row already exists.

This two-step is critical for idempotency. See
[`schema/conventions.md`](../schema/conventions.md#batch-writes-and-idempotency).

## Liquidity rewards

`indexLiquidityReward` is structurally identical but skips
`classifyReward` — the type is hardcoded as `RewardTypeLiquidity`.

## Historical reward bug

For most of the indexer's lifetime, the reward-detection branch was
silently broken because the code used literal `BlockType` ints from
the Dart port that didn't match the Go SDK's enum values:

- Pre-fix: `block.BlockType == 4 && block.PairedAccountBlock.BlockType == 6`
- Post-fix: `block.BlockType == BlockTypeUserReceive (3) && block.PairedAccountBlock.BlockType == BlockTypeContractSend (4)`

The fix is in migration timeline; the historical-data backfill lives at
[`scripts/backfill-rewards/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards).
Run it once against any DB that has pre-fix data; thereafter the live
indexer keeps the tables current.

## Tests

- [`internal/indexer/rewards_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards_test.go) — unit tests for `determineRewardType`.
- [`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go) — `TestIntegration_Reward_CumulativeRewardsAccumulates` for the additive aggregation.
- [`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go) — `TestIntegration_Pillar_IsWithdrawAddress` covers the lookup that drives the pillar/delegation split.
