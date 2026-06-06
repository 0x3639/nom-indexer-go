---
title: htlcs
---

# `htlcs`

## Purpose

One row per HTLC (hash-time-locked contract) entry. An HTLC is a conditional
transfer: the **time-locked** party (the sender) **Creates** an entry that
locks an amount behind a hash and an expiry. The **hash-locked** party can
**Unlock** it by revealing a preimage whose hash matches `hash_lock`, claiming
the funds. If nobody unlocks before `expiration_timestamp`, the time-locked
party can **Reclaim** the funds. The lifecycle is therefore
`Create → (Unlock | Reclaim)`: an entry starts `status = active` and is settled
to `unlocked` or `reclaimed` exactly once.

## Columns

All 15 columns from
[`migrations/014_htlcs.up.sql`](https://github.com/0x3639/nom-indexer-go/blob/main/migrations/014_htlcs.up.sql).
Amounts are int64 `BIGINT`; timestamps are Unix seconds; hashes/addresses
follow the [schema conventions](conventions.md).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. The `Create` send-block hash (`paired.Hash`, = go-zenon `sendBlock.Hash`). See **Notes** for the id-linkage convention. |
| `time_locked_address` | `TEXT` | NO | `''` | Sender (`z1…`); can `Reclaim` after expiry. |
| `hash_locked_address` | `TEXT` | NO | `''` | Recipient (`z1…`); can `Unlock` with the preimage. |
| `token_standard` | `TEXT` | NO | `''` | Locked token (`zts1…`). |
| `amount` | `BIGINT` | NO | `0` | Locked amount. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `expiration_timestamp` | `BIGINT` | NO | `0` | Unix seconds. After this the entry is reclaimable. |
| `hash_type` | `SMALLINT` | NO | `0` | Hash algorithm: `0` = SHA3, `1` = SHA256. |
| `key_max_size` | `SMALLINT` | NO | `0` | Maximum preimage length accepted by the contract. |
| `hash_lock` | `TEXT` | NO | `''` | Hex-encoded hash the preimage must match. |
| `status` | `SMALLINT` | NO | `0` | Lifecycle: `0` = active, `1` = unlocked, `2` = reclaimed. |
| `preimage` | `TEXT` | NO | `''` | Hex-encoded preimage; set on `Unlock`, empty otherwise. |
| `creation_momentum_height` | `BIGINT` | NO | `0` | Momentum height of the `Create`. |
| `creation_momentum_timestamp` | `BIGINT` | NO | `0` | Unix seconds of the `Create` momentum. |
| `settle_momentum_height` | `BIGINT` | NO | `0` | Momentum height of the `Unlock`/`Reclaim`; `0` while active. |
| `settle_momentum_timestamp` | `BIGINT` | NO | `0` | Unix seconds of the settling momentum; `0` while active. |

## Enums

| Column | Value | Meaning |
|---|---|---|
| `status` | `0` | active (created, not yet settled) |
| `status` | `1` | unlocked (claimed by `hash_locked_address` with a preimage) |
| `status` | `2` | reclaimed (returned to `time_locked_address` after expiry) |
| `hash_type` | `0` | SHA3 |
| `hash_type` | `1` | SHA256 |

The Go constants are `HtlcStatusActive` / `HtlcStatusUnlocked` /
`HtlcStatusReclaimed` in
[`internal/models/models.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/models.go).

## Primary key & indexes

- **Primary key:** `id`.
- `idx_htlcs_time_locked` (`time_locked_address`),
  `idx_htlcs_hash_locked` (`hash_locked_address`),
  `idx_htlcs_status` (`status`).

## Relations

- `time_locked_address`, `hash_locked_address` ↔ [`accounts.address`](accounts.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `id` is the `Create` **send-block** hash (`paired.Hash`) — see
  [`account_blocks`](account_blocks.md) and the **Notes** section below.

## Write path

All writes come from
[`indexHtlcContract`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go),
per account block, against the HTLC contract address
`z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw`:

- **`InsertBatch`** on a `Create` method — populates the lock params, the
  sender (`time_locked_address`), the recipient (`hash_locked_address` from the
  ABI input), and the paired send's amount/token. `status = active`.
- **`SettleBatch`** on an `Unlock` method — looks up by `id` (the decoded
  `id` input), sets `status = unlocked`, stores the `preimage`, and records the
  settle momentum.
- **`SettleBatch`** on a `Reclaim` method — looks up by `id`, sets
  `status = reclaimed`, and records the settle momentum.

## Read patterns

- **Active HTLCs** — `WHERE status = 0`.
- **HTLCs for an address** — `WHERE time_locked_address = $1 OR hash_locked_address = $1`.
- **HTLC by id** — direct PK lookup.

## Notes

### id-linkage convention (verified against live chain data, Task 1.6)

`Unlock` and `Reclaim` settle an entry by matching their decoded `id` ABI
input to the `htlcs.id` stored at `Create`. The two must use the **same** id or
settlements silently no-op (`SettleBatch` updates zero rows).

Live mainnet data was available and verified against the synced
`nom_indexer` DB (frontier). HTLC usage on Zenon is **not** sparse:

| Method | Account blocks |
|---|---|
| `Create` | 560 |
| `Unlock` | 509 |
| `Reclaim` | 62 |

The HTLC id is the **send-block hash**. go-zenon assigns
`HtlcInfo.Id = sendBlock.Hash` when it processes a `Create`
(`vm/embedded/implementation/htlc.go`), and `Unlock`/`Reclaim` look the entry up
by that id. In handler terms the `Create` arrives as a contract-receive block
(`block`) whose `block.PairedAccountBlock` (`paired`) is the user's send block,
so the id is **`paired.Hash`**.

Empirical confirmation across **all 571** settle blocks on mainnet (the
`Create` rows are `block_type = 2` user-send blocks, so `account_blocks.hash`
on a `Create` row is the send-block hash = `paired.Hash`):

- **509/509** `Unlock` and **62/62** `Reclaim` blocks have `input.id` equal to
  the `Create` **send-block hash** (`paired.Hash`).
- **0/571** match the `Create` contract-receive block hash (`block.Hash`, the
  `paired_account_block` of the receive block).

This matches [`stakes`](stakes.md) / [`fusions`](fusions.md) in using the
send-block hash as the id, but unlike them HTLC needs no separately ABI-derived
`cancel_id` — `Unlock`/`Reclaim` carry the send-block hash directly as
`input.id`. The handler stores `id = paired.Hash` on `Create`, so `SettleBatch`
updates the correct row and entries transition out of `active` when settled.
