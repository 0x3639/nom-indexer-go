---
title: Liquidity contract
---

# Liquidity contract

## Contract address

`z1qxemdeddedxlyquydytyxxxxxxxxxxxxflaaae` — `LiquidityAddress`.

Treasury used for reward routing:
`z1qqw8f3qxx9zg92xgckqdpfws3dw07d26afsj74` — `LiquidityTreasuryAddress`.

## Methods observed

There is **no per-method handler** for the Liquidity contract. Liquidity
indexing is reward-only — the indexer detects receive blocks paired with
sends from the Liquidity treasury and routes them through
`indexLiquidityReward` (`rewards.go`).

## Per-method write effects

- **Receive paired with treasury send** (detected in `processAccountBlocks`)
    - `reward_transactions`: row classified as `RewardTypeLiquidity` (3).
    - `cumulative_rewards`: additive increment for
      `(address, RewardTypeLiquidity, token_standard)`.

## Special computation

The detection condition is:

```
block.BlockType == utils.BlockTypeUserReceive (3)
AND block.PairedAccountBlock != nil
AND block.PairedAccountBlock.Address == LiquidityTreasuryAddress
```

The amount and token come from the paired send block, not from a decoded
ABI input — there is no method call to decode.

## Tests

No dedicated unit test. Liquidity rewards are exercised end-to-end via
the `backfill-rewards` script and via the per-momentum batch tests.

## Notes

Liquidity rewards historically populated the `reward_transactions` table
even when other reward types didn't, because the treasury-source check
doesn't depend on the `ContractSend` BlockType literal that was the
source of the historical reward-detection bug (see
[`rewards.md`](rewards.md)). The
[`docs/reference/known-issues.md`](../reference/known-issues.md) page
covers the history.
