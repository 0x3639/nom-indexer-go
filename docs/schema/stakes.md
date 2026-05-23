---
title: stakes
---

# `stakes`

## Purpose

One row per `Stake.Stake` event. Tracks the staked amount, duration, expiry,
the originating address, and the ABI-derived `cancel_id` that lets us
correlate a future `Cancel` call with the stake it cancels.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. The stake's account-block hash. |
| `address` | `TEXT` | NO | — | Staker (`z1…`). |
| `start_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `expiration_timestamp` | `BIGINT` | NO | — | `start_timestamp + duration_in_sec`. |
| `znn_amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `duration_in_sec` | `INT` | NO | — | Lock duration. |
| `is_active` | `BOOLEAN` | NO | — | False once the stake has been cancelled. |
| `cancel_id` | `TEXT` | NO | — | 64-char hex. Derived by ABI-encoding `Cancel(id)` against the Stake contract. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_stakes_address`, `idx_stakes_is_active`.

## Relations

- `address` ↔ [`accounts.address`](accounts.md).
- `cancel_id` is the ABI-encoded `Cancel` parameter, used to match the
  `Cancel` event to the stake.

## Write path

- **`InsertBatch`** from `indexStakeContract` on a `Stake` method, in
  [`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).
  The `id` is the paired send block's hash; `cancel_id` is computed via
  `getStakeCancelID` (encodes `Cancel(id)` through the SDK ABI).
- **`SetInactiveBatch`** from a `Cancel` method handler — looks up by
  `(cancel_id, address)`.

## Read patterns

- **Active stake count + ZNN staked** — `WHERE is_active = true`. Used by
  network-stats snapshot.
- **Stakes by address** — `WHERE address = $1`.
- **Stake by ID** — direct PK lookup.

## Gotchas

- `cancel_id` derivation depends on the SDK's ABI tables being correct.
  If a Cancel arrives but `is_active` doesn't flip, the most likely cause
  is a mismatched cancel_id (re-derive with `getStakeCancelID` and compare).
- `is_active` is not the same as "still locked". A stake whose
  `expiration_timestamp` has passed but hasn't been collected is still
  `is_active = true` until the on-chain `Cancel`/`Collect` fires.
