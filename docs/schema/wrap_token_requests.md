---
title: wrap_token_requests
---

# `wrap_token_requests`

## Purpose

A user's intent to wrap a Zenon ZTS token onto an external chain (Ethereum,
BSC, etc.). Polled from `BridgeApi.GetAllWrapTokenRequests` on a 1-minute
cadence, paging newest-first until we reach the oldest unfinalized row we
already have.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. Wrap request hash. |
| `network_class` | `INT` | NO | — | Bridge network class (e.g., 2 = EVM). |
| `chain_id` | `INT` | NO | — | Target chain ID (Ethereum mainnet = 1). |
| `to_address` | `TEXT` | NO | — | External-chain recipient (raw — format varies by network). |
| `token_standard` | `TEXT` | NO | — | ZTS being wrapped. |
| `token_address` | `TEXT` | NO | — | External-chain token contract. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `fee` | `BIGINT` | NO | — | Bridge fee in the same token. |
| `signature` | `TEXT` | NO | — | Orchestrator signature once provided; empty until then. |
| `creation_momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `confirmations_to_finality` | `INT` | NO | `0` | Confirmations remaining on the destination chain. `0` = finalized. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_wrap_token_requests_to_address`,
  `idx_wrap_token_requests_token_standard`,
  `idx_wrap_token_requests_chain` (composite on `network_class, chain_id`),
  `idx_wrap_token_requests_creation_height`.

## Relations

- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `(network_class, chain_id)` ↔ [`bridge_networks`](bridge_networks.md) and
  [`bridge_network_tokens`](bridge_network_tokens.md).
- `creation_momentum_height` ↔ [`momentums.height`](momentums.md).

## Write path

`updateBridgeWrapRequests` in
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
called on the 1-minute bridge sync. The paging strategy:

1. Look up the oldest **unfinalized** wrap height in the DB
   (`GetWrapSyncStopHeight`) — that's our floor.
2. Fetch pages newest-first. Upsert each row.
3. Stop once we see a row at or below the floor.

`UpsertWrapRequest` updates only `signature` and `confirmations_to_finality`
on conflict — the rest of the fields are immutable once the request exists.

## Read patterns

- **Wrap by ID** — direct PK lookup.
- **Pending wraps** — `WHERE confirmations_to_finality > 0 ORDER BY
  creation_momentum_height DESC`.
- **Wraps to an external address** — `WHERE to_address = $1`.
- **Daily wrap volume per network/token** — `WHERE creation_momentum_height
  IN (heights in date range) GROUP BY network_class, chain_id, token_standard`.

## Gotchas

- `to_address` is a raw string — for EVM networks it's a `0x…` hex
  address; for other network classes it may be a different shape. Treat
  as opaque unless you know the network class.
- `confirmations_to_finality` of 0 means **finalized**, not "no
  confirmations needed". The semantic flip is "count down to zero".
- `signature` is empty until an orchestrator signs the request — pending
  rows are normal during normal bridge operation.
- The 1-minute polling cadence means new wraps can lag by up to a minute
  appearing in this table even though the underlying account block is
  already in [`account_blocks`](account_blocks.md).
