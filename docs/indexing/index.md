---
title: Indexing flow
---

# Indexing flow

How a momentum becomes rows. Starting at the wire and ending at committed
Postgres transactions.

## High level

1. **Receive a momentum.** The indexer subscribes to
   `SubscriberApi.ToMomentums`. On each momentum, plus during catch-up
   sync, it calls `LedgerApi.GetMomentumsByHeight` to fetch the
   block with its full account-block content.
2. **Build a pgx batch.** `processMomentum` in
   [`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go)
   creates a `pgx.Batch` and queues every write for the momentum.
3. **Loop the account blocks.**
   `processAccountBlocks` iterates `m.Content`. For each block:
    - Fetch the full block via `LedgerApi.GetAccountBlockByHash`.
    - Decode ABI inputs if the block targets an embedded contract
      (see [`abi-decoding.md`](abi-decoding.md)).
    - Upsert the [`accounts`](../schema/accounts.md) row + flow metrics.
    - Insert the [`account_blocks`](../schema/account_blocks.md) row.
    - Dispatch to a per-contract handler if applicable (this page).
    - Detect reward-receive blocks and route through
      [`rewards.md`](rewards.md).
    - Update [`tokens`](../schema/tokens.md) if `TokenInfo` is present.
4. **Update balances.** Skipped when `tx_count >= 1000` (genesis safety
   valve). Otherwise calls `GetAccountInfoByAddress` per touched address
   and upserts [`balances`](../schema/balances.md).
5. **Queue producer + momentum writes.** The producer pillar's
   `produced_momentum_count` increment and the parent `momentums` row
   are queued at the end of the batch.
6. **Commit or roll back.** `processMomentum` opens the transaction
   immediately before `SendBatch`. On any per-op error inside the
   batch, the whole transaction rolls back and the sync loop retries
   the height.

## Contract dispatch

`indexEmbeddedContracts` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go)
switches on the embedded contract address and routes to one of:

| Contract | Handler | Page |
|---|---|---|
| Pillar | `indexPillarContract` | [pillar-contract.md](pillar-contract.md) |
| Sentinel | `indexSentinelContract` | [sentinel-contract.md](sentinel-contract.md) |
| Stake | `indexStakeContract` | [stake-contract.md](stake-contract.md) |
| Plasma | `indexPlasmaContract` | [plasma-contract.md](plasma-contract.md) |
| Accelerator | `indexAcceleratorContract` | [accelerator-contract.md](accelerator-contract.md) |
| Token | `indexTokenContract` | [token-contract.md](token-contract.md) |
| Liquidity | (reward-only — no method handler) | [liquidity-contract.md](liquidity-contract.md) |
| Bridge | `updateBridgeWrapRequests` / `updateBridgeUnwrapRequests` | [bridge-contract.md](bridge-contract.md) |

Each handler matches on the decoded `txData.Method` and performs the
appropriate batched writes.

## Cross-cutting helpers

- **`getPillarOwnerAddress(name)`** — looks up the owner from the cached
  pillar map (populated by `updateCachedData`).
- **`getPillarInfoForProducer(producer, height)`** — historical resolution
  of (producer → owner, name) via
  [`pillar_updates`](../schema/pillar_updates.md), falling back to the
  current [`pillars`](../schema/pillars.md) row.
- **`getVotingID(id)`** — ABI encode/decode round-trip on the Accelerator
  ABI to derive the voting_id for a project or phase.
- **`getStakeCancelID(id)` / `getFusionCancelID(id)`** — same idea for
  cancel IDs against the Stake / Plasma ABIs.

## Reward classification

Reward indexing is its own subsystem because the BlockType-based
detection bug history made it the most fragile area of the indexer. See
[`rewards.md`](rewards.md) for the full story.

## Idempotency

Every per-contract write uses one of three patterns:

- `ON CONFLICT … DO NOTHING` (event-keyed rows, e.g.,
  [`token_mints`](../schema/token_mints.md))
- `ON CONFLICT … DO UPDATE SET …` (state rows, e.g.,
  [`pillars`](../schema/pillars.md))
- Additive `amount = existing + EXCLUDED` (counters in
  [`cumulative_rewards`](../schema/cumulative_rewards.md))

The first two are safe to retry; the third is **only** safe inside the
original transaction, because retrying after the contributing
event-keyed row was committed would double-count. See
[`schema/conventions.md`](../schema/conventions.md#batch-writes-and-idempotency).
