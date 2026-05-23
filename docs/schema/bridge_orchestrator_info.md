---
title: bridge_orchestrator_info
---

# `bridge_orchestrator_info`

## Purpose

Singleton table holding the bridge orchestrator's operating parameters
pulled from `BridgeApi.GetOrchestratorInfo`. Orchestrators are the
off-chain TSS signers that move funds between Zenon and external chains;
their cadence and quorum thresholds live here.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `row_id` | `SMALLINT` | NO | — | Primary key, `CHECK (row_id = 1)`. |
| `window_size` | `BIGINT` | NO | `0` | Sliding window for orchestrator timing. |
| `key_gen_threshold` | `INT` | NO | `0` | Minimum signers for key generation. |
| `confirmations_to_finality` | `INT` | NO | `0` | External-chain confirmations required before a wrap is finalized. |
| `estimated_momentum_time` | `INT` | NO | `0` | Seconds-per-momentum assumed by orchestrators. |
| `allow_key_gen_height` | `BIGINT` | NO | `0` | Momentum height after which key generation is allowed. |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds. |

## Primary key & indexes

- **Primary key:** `row_id` with `CHECK (row_id = 1)`.

## Relations

- Standalone — no incoming references.

## Write path

`updateBridgeConfig` — single upsert per bridge sync tick.

## Read patterns

- **Current orchestrator config** — `SELECT * FROM bridge_orchestrator_info
  LIMIT 1`.

## Gotchas

- Singleton — same `row_id = 1` constraint as
  [`bridge_admin`](bridge_admin.md).
- `confirmations_to_finality` here is the **bridge-wide** target;
  individual wrap requests track their **remaining** confirmations in
  [`wrap_token_requests.confirmations_to_finality`](wrap_token_requests.md).
