---
title: bridge_admin
---

# `bridge_admin`

## Purpose

Singleton table — exactly one row keyed `row_id = 1` — holding the bridge
administrator address plus the bridge-wide TSS pubkey + halt flags from
`BridgeApi.GetBridgeInfo`.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `row_id` | `SMALLINT` | NO | — | Primary key. Checked `= 1`. |
| `administrator` | `TEXT` | NO | — | Current bridge admin address. |
| `compressed_tss_ecdsa_pubkey` | `TEXT` | NO | `''` | TSS-controlled pubkey, compressed. |
| `decompressed_tss_ecdsa_pubkey` | `TEXT` | NO | `''` | Same, uncompressed. |
| `allow_key_gen` | `BOOLEAN` | NO | `false` | Whether key generation is currently permitted. |
| `halted` | `BOOLEAN` | NO | `false` | Whether the bridge is halted. |
| `unhalted_at` | `BIGINT` | NO | `0` | Momentum height the bridge was last unhalted. |
| `unhalt_duration_in_momentums` | `BIGINT` | NO | `0` | Duration of the unhalt window. |
| `tss_nonce` | `BIGINT` | NO | `0` | Current TSS nonce. |
| `metadata` | `TEXT` | NO | `''` | Opaque metadata. |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds of last refresh. |

## Primary key & indexes

- **Primary key:** `row_id` with `CHECK (row_id = 1)` — enforces singleton.

## Relations

- `administrator` ↔ [`accounts.address`](accounts.md).

## Write path

`updateBridgeConfig` — calls `BridgeApi.GetBridgeInfo` and upserts the
single row with `row_id = 1`.

## Read patterns

- **Current admin** — `SELECT * FROM bridge_admin WHERE row_id = 1` (or
  just `LIMIT 1`).
- **Halt status** — `SELECT halted FROM bridge_admin LIMIT 1`. Used by
  monitoring.

## Gotchas

- Always one row, always `row_id = 1`. Attempting to insert any other row
  ID is blocked by the CHECK constraint.
- The bridge can be halted by a guardian quorum — see
  [`bridge_guardians`](bridge_guardians.md) for the active set.
