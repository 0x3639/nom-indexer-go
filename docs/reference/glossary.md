---
title: Glossary
---

# Glossary

Zenon-specific terms used throughout this codebase and the docs.

## Account block

The base unit of activity on the Zenon dual ledger. Each block belongs
to a specific address (the sender) and may carry a send or a receive.
See [`schema/account_blocks.md`](../schema/account_blocks.md).

## Accelerator-Z

The on-chain funding mechanism. Projects ask for ZNN + QSR; pillars
vote yes/no; phases are sub-grants within a project. Indexed as
[`projects`](../schema/projects.md),
[`project_phases`](../schema/project_phases.md),
[`votes`](../schema/votes.md).

## ABI

Application Binary Interface — the encoding spec for contract method
calls. The SDK ships ABI tables for every embedded contract; the
indexer's [`decoder.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder.go)
matches method signatures against them.

## Bech32

The address-encoding scheme. Zenon addresses always start with `z1`;
ZTS token standards start with `zts1`.

## Cancel ID

ABI-derived ID computed by encoding `Cancel(<id>)` (or `CancelFuse(<id>)`)
against the relevant contract's ABI. Lets the indexer match a later
cancel event to the stake / fusion it cancels without storing both
the original ID and the cancel ID separately. See
[`schema/stakes.md`](../schema/stakes.md) and
[`schema/fusions.md`](../schema/fusions.md).

## Contract receive / send

In Zenon's dual-ledger model, calls into embedded contracts produce
a pair of blocks: a `ContractSend` from the caller and a
`ContractReceive` on the contract's address. The indexer routes
events by `block.Address` on the receive side. See
[`indexing/index.md`](../indexing/index.md).

## Delegation

When an account assigns its voting weight to a pillar without
transferring tokens. Tracked currently in
[`accounts.delegate`](../schema/accounts.md) (single value) and
historically in [`delegations`](../schema/delegations.md)
(time-bucketed intervals).

## Embedded contract

A built-in contract baked into the Zenon protocol. There are nine:
Pillar, Sentinel, Stake, Plasma, Accelerator, Token, Swap, Liquidity,
Bridge, plus the Spork governance contract. Addresses listed in
[`reference/addresses.md`](addresses.md).

## Epoch

A fixed window of blocks (currently ~one day) used by the protocol
for reward accounting. The indexer doesn't track epoch boundaries
directly — it tracks per-momentum events.

## Frontier momentum

The latest block the node knows about. The indexer's catch-up sync
runs until `MAX(momentums.height) >= frontier.height`, then switches
to real-time subscription.

## Genesis momentum / genesis receive

The chain's first block (height 1). Carries initial address balances
via `BlockTypeGenesisReceive` blocks. The indexer treats it specially:
the genesis block has thousands of tx_count and per-address balance
refresh is skipped (would take hours).

## Hash

64-character lowercase hex string (no `0x` prefix). Used for
momentum IDs, account block IDs, stake / fusion / project / phase
IDs, voting IDs, and cancel IDs. See
[`schema/conventions.md`](../schema/conventions.md#hash-encoding).

## Momentum

The Zenon term for a block. The chain produces one roughly every 10
seconds. See [`schema/momentums.md`](../schema/momentums.md).

## Network of Momentum (NoM)

The Zenon Network's full name. "NoM" appears in some upstream code
and docs.

## Orchestrator

An off-chain bridge participant who runs TSS signing for wrap/unwrap
operations. Tracked indirectly via
[`bridge_orchestrator_info`](../schema/bridge_orchestrator_info.md)
(the singleton config).

## Paired account block

The counterpart of a send or receive — for every `Send` there's a
corresponding `Receive` and vice versa. Used heavily in reward
detection. See [`indexing/rewards.md`](../indexing/rewards.md).

## Pillar

A validator that produces blocks and earns block rewards. ~30–50 on
test net. See [`schema/pillars.md`](../schema/pillars.md).

## Plasma

Zenon's compute-credit system. Users "fuse" QSR to grant plasma to
themselves or a beneficiary; the plasma is consumed by sending
transactions. See [`schema/fusions.md`](../schema/fusions.md).

## QSR

Zenon's secondary token, `zts1qsrxxxxxxxxxxxxxmrhjll`. Used for
plasma fusion and as a reward token alongside ZNN.

## Sentinel

A lower-tier participant that earns sentinel-class rewards without
producing blocks. See [`schema/sentinels.md`](../schema/sentinels.md).

## Slot cost (QSR)

The QSR burned to spawn a pillar. Indexed when the indexer detects
a Burn descendant of the Register block. Stored in
`pillars.slot_cost_qsr`.

## Spawn timestamp

Unix seconds when a pillar was registered. Used by the voting
activity cron to determine which proposals a pillar was eligible to
vote on.

## TSS

Threshold signature scheme — multi-party signing used by bridge
orchestrators. The bridge admin tracks the compressed + decompressed
TSS ECDSA pubkey.

## TSS nonce

A monotonically increasing nonce used to order TSS signatures.

## Unwrap

Bridge action: external chain → Zenon. A user burns a wrapped token
on the external chain; an orchestrator emits the redeemable
[`unwrap_token_requests`](../schema/unwrap_token_requests.md) row on
Zenon.

## Voting ID

ABI-derived ID used by Accelerator-Z votes. Pillars vote against the
voting ID, not the underlying project/phase ID. Computed via
`getVotingID` in `indexer.go`. See
[`indexing/accelerator-contract.md`](../indexing/accelerator-contract.md).

## Wrap

Bridge action: Zenon → external chain. A user sends a ZTS to the
Bridge contract; orchestrators sign and the wrapped token appears on
the external chain. Tracked in
[`wrap_token_requests`](../schema/wrap_token_requests.md).

## ZNN

Zenon's primary token, `zts1znnxxxxxxxxxxxxx9z4ulx`. Used for staking,
delegation, and as the dominant reward token.

## ZTS — Zenon Token Standard

The token format on Zenon. Every token has a `zts1…` identifier.
The empty token standard `zts1qqqqqqqqqqqqqqqqtq587y` is a sentinel
meaning "no token transfer".
