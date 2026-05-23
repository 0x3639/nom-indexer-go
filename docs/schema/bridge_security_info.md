---
title: bridge_security_info
---

# `bridge_security_info`

## Purpose

Singleton with the bridge security delay parameters from
`BridgeApi.GetSecurityInfo`. The administrator delay and the soft delay
control how long admin actions must wait before taking effect.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `row_id` | `SMALLINT` | NO | — | Primary key, `CHECK (row_id = 1)`. |
| `administrator_delay` | `BIGINT` | NO | `0` | Seconds an admin change must wait before applying. |
| `soft_delay` | `BIGINT` | NO | `0` | Soft-action delay (less critical operations). |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds. |

## Primary key & indexes

- **Primary key:** `row_id` with `CHECK (row_id = 1)`.

## Relations

- Standalone — no incoming references.

## Write path

`updateBridgeConfig` — single upsert per bridge sync tick.

## Read patterns

- **Current security parameters** — `SELECT * FROM bridge_security_info
  LIMIT 1`.

## Gotchas

- Singleton with the same `row_id = 1` invariant.
- Guardians are **not** stored here — see
  [`bridge_guardians`](bridge_guardians.md). This table is just the delay
  knobs.
