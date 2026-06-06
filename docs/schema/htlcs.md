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
| `id` | `TEXT` | NO | — | Primary key. The creating (`Create`) account-block hash. See **Notes** for the id-linkage convention. |
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
- `id` is the `Create` account-block hash — see [`account_blocks`](account_blocks.md)
  and the **Notes** section below.

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

The empirical result, checked across **all** 571 settle blocks:

- For **509/509** `Unlock` and **62/62** `Reclaim` blocks, the decoded
  `input.id` equals the **`Create` receive-block's own `hash`**
  (`account_blocks.hash` = `block.Hash`), i.e. the contract-receive block on
  the HTLC address.
- It does **not** equal the `Create` block's `paired_account_block` (the
  user's send-block hash): **0/509** `Unlock` and **0/62** `Reclaim` match
  that value.

So the canonical HTLC id is the **`Create` contract-receive block hash**, not
the paired send-block hash. This differs from [`stakes`](stakes.md) /
[`fusions`](fusions.md), where the id is the paired send-block hash and the
settling call is correlated through a separately ABI-derived `cancel_id`.

> **Fix applied:** the handler now stores `id = block.Hash` (the `Create`
> receive-block hash) on `Create`, which is the value the live data confirms
> `Unlock`/`Reclaim` reference — verified against **571/571** mainnet
> settlements (509 `Unlock` + 62 `Reclaim`, 100% match). With this id,
> `SettleBatch` updates the correct row, so entries transition out of `active`
> when settled. (Earlier revisions stored `paired.Hash`, the send-block hash,
> under which settlements matched no rows; that defect is now corrected.)
