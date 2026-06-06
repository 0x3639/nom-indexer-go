---
title: swap_retrievals
---

# `swap_retrievals`

## Purpose

One row per legacy genesis-swap **RetrieveAssets** claim. At network genesis,
ZNN and QSR balances from the legacy (pre-Zenon) chain were escrowed in the
embedded **Swap** contract, keyed by a public key. A holder claims their
allocation by calling `RetrieveAssets(publicKey, signature)` on the swap
contract; the contract verifies the signature and disburses the claimant's
remaining genesis ZNN and/or QSR back to their address.

This is a **near-static historical dataset** — the vast majority of swap
claims happened shortly after genesis, and no new genesis allocations are ever
created. Each row records who claimed, with which public key, and how much ZNN
and QSR they received in that claim.

## Columns

All 7 columns from
[`migrations/015_swap.up.sql`](https://github.com/0x3639/nom-indexer-go/blob/main/migrations/015_swap.up.sql).
Amounts are int64 `BIGINT`; timestamps are Unix seconds; hashes/addresses
follow the [schema conventions](conventions.md).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. The claimant's paired send-block hash (the `RetrieveAssets` send). |
| `address` | `TEXT` | NO | `''` | Recipient/claimant (`z1…`). |
| `public_key` | `TEXT` | NO | `''` | The legacy public key presented in the `RetrieveAssets` ABI input. |
| `znn_amount` | `BIGINT` | NO | `0` | ZNN disbursed in this claim. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `qsr_amount` | `BIGINT` | NO | `0` | QSR disbursed in this claim. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `momentum_height` | `BIGINT` | NO | `0` | Momentum height of the `RetrieveAssets` claim. |
| `momentum_timestamp` | `BIGINT` | NO | `0` | Unix seconds of the claim momentum. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_swap_retrievals_address` (`address`).

## Relations

- `address` ↔ [`accounts.address`](accounts.md).
- `id` is the claimant's send-block hash — see [`account_blocks`](account_blocks.md).

## Write path

All writes come from
[`indexSwapContract`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go),
per account block, against the Swap contract address
`z1qxemdeddedxswapxxxxxxxxxxxxxxxxxxl4yww`:

- **`InsertRetrievalBatch`** on a `RetrieveAssets` method. The handler keys the
  row by the **paired send-block hash** (`paired.Hash`) and takes `address`
  from the paired send (`paired.Address`) and `public_key` from the decoded
  `publicKey` ABI input.
- The handler skips any block whose method is not `RetrieveAssets` or that has
  no paired account block.

## Read patterns

- **Claims by address** — `WHERE address = $1`.
- **Claim by id** — direct PK lookup.

## Notes

`znn_amount` and `qsr_amount` are **decoded from the descendant
`Token.Mint` calls**, not read from the descendant block amounts. A
`RetrieveAssets` receive on the swap contract spawns descendant blocks
(`block.DescendantBlocks`) that disburse the claimant's remaining genesis
balance — but go-zenon emits each as a `Mint` call to the token contract whose
block-level `Amount` is `0` and whose block-level `TokenStandard` is **ZNN for
both** the ZNN and QSR payout (a quirk in
`vm/embedded/implementation/swap.go`). The real token and amount live in the
`Mint(tokenStandard, amount, receiveAddress)` call data, so the handler decodes
each descendant's `Data` with the Token ABI and, for `Mint` calls, accumulates
the decoded `amount` into `znn_amount` or `qsr_amount` by matching the Mint's
`tokenStandard`. A claim that disburses only one asset leaves the other amount
at `0`. (Reading the descendant block `.Amount`/`.TokenStandard` directly would
record `0` and mis-bucket QSR as ZNN.)

The **authoritative remaining (unclaimed) balances** live in the
[`swap_assets`](swap_assets.md) snapshot, not here — this table is the per-claim
event log.
