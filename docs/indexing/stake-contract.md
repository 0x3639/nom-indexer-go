---
title: Stake contract
---

# Stake contract

## Contract address

`z1qxemdeddedxstakexxxxxxxxxxxxxxxxjv8v62` — `StakeAddress`.

## Methods observed

Handler: `indexStakeContract` in `embedded.go`.

| Method | Inputs | Triggers |
|---|---|---|
| `Stake` | `durationInSec` | Insert into [`stakes`](../schema/stakes.md). |
| `Cancel` | `id` | Mark the stake inactive. |

## Per-method write effects

- **Stake**
    - Reads the stake ID from `block.PairedAccountBlock.Hash`.
    - `duration_in_sec` is parsed from the decoded `durationInSec` input
      (defaults to 0 on parse error).
    - `znn_amount` is the paired send block's `Amount`.
    - `expiration_timestamp = momentum_timestamp + duration_in_sec`.
    - `cancel_id` is computed via `getStakeCancelID(stakeID)` — ABI-encode
      `Cancel(id)` against the Stake contract and read back the first
      parameter.
    - `stakes`: `InsertBatch` (idempotent via `ON CONFLICT (id) DO NOTHING`).
- **Cancel**
    - The input `id` is **the original stake ID**, not the cancel ID.
    - The handler computes the cancel ID from the input via
      `getStakeCancelID(id)`, then `SetInactiveBatch(cancel_id, address)`.

## Special computation

`cancel_id` is the ABI-encoded `Cancel(id)` call's first parameter,
read back through the SDK ABI tables. The roundtrip is deterministic —
two calls with the same input always produce the same cancel_id.

## Tests

- [`internal/indexer/voting_id_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/voting_id_test.go)
  exercises `getStakeCancelID` via the shared roundtrip helper test.

## Notes

Stake-source rewards land in [`reward_transactions`](../schema/reward_transactions.md)
as `RewardTypeStake` (1). See [`rewards.md`](rewards.md).
