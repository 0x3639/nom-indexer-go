---
title: fusions
---

# `fusions`

## Purpose

Plasma fusion entries — QSR fused (locked) to grant plasma to a beneficiary
address. Mirror image of [`stakes`](stakes.md) for the plasma contract,
including the ABI-derived `cancel_id`.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. Fusion's account-block hash. |
| `address` | `TEXT` | NO | — | Account that fused (paid the QSR). |
| `beneficiary` | `TEXT` | NO | — | Address receiving the plasma; may equal `address`. |
| `momentum_hash` | `TEXT` | NO | — | Joins to [`momentums.hash`](momentums.md). |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `qsr_amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `expiration_height` | `BIGINT` | NO | — | Approximate height at which the fusion can be cancelled; `momentum_height + FusionExpirationBlocks` (≈ 1 hour @ 10s blocks). |
| `is_active` | `BOOLEAN` | NO | — | False after `CancelFuse`. |
| `cancel_id` | `TEXT` | NO | — | ABI-encoded `CancelFuse(id)` parameter. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_fusions_address`, `idx_fusions_beneficiary`, `idx_fusions_is_active`.

## Relations

- `address`, `beneficiary` ↔ [`accounts.address`](accounts.md).
- `momentum_*` ↔ [`momentums`](momentums.md).

## Write path

- **`InsertBatch`** from `indexPlasmaContract` on a `Fuse` method, in
  [`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).
  `id` is the paired send block's hash; `cancel_id` is `getFusionCancelID(id)`;
  `expiration_height` adds `FusionExpirationBlocks` (defined in
  [`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go)).
- **`SetInactiveBatch`** from `CancelFuse` — same `(cancel_id, address)`
  pattern as stakes.

## Read patterns

- **Active fusion count + QSR fused** — `WHERE is_active = true`.
- **Fusions for a beneficiary** — `WHERE beneficiary = $1`.
- **About to expire** — `WHERE is_active = true AND expiration_height <= $h`.

## Gotchas

- `expiration_height` is **approximate** — it's the current momentum height
  plus a fixed block-time constant (`FusionExpirationBlocks ≈ 360` blocks
  for a 1-hour window at 10s/block). The chain's actual expiry is enforced
  on the contract side; this column is a UI hint, not an authoritative
  expiry time.
- `beneficiary` defaults to the sender if no `address` input is supplied in
  the Fuse call.
- Same `cancel_id` correctness note as stakes — if `Cancel` doesn't flip
  `is_active`, suspect a cancel_id derivation mismatch.
