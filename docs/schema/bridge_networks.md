---
title: bridge_networks
---

# `bridge_networks`

## Purpose

Cached configuration of each destination network the Zenon bridge is
aware of. Refreshed from `BridgeApi.GetAllNetworks` on the 1-minute bridge
sync.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `network_class` | `INT` | NO | — | Part of composite PK. e.g., 2 = EVM. |
| `chain_id` | `INT` | NO | — | Part of composite PK. Chain ID (Ethereum mainnet = 1). |
| `name` | `TEXT` | NO | — | Display name. |
| `contract_address` | `TEXT` | NO | — | Bridge contract address on the external chain. |
| `metadata` | `TEXT` | NO | `''` | Opaque metadata blob from the contract. |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds of last refresh. |

## Primary key & indexes

- **Primary key:** `(network_class, chain_id)`.

## Relations

- `(network_class, chain_id)` ↔ [`bridge_network_tokens`](bridge_network_tokens.md),
  [`wrap_token_requests`](wrap_token_requests.md),
  [`unwrap_token_requests`](unwrap_token_requests.md),
  [`bridge_stat_histories`](bridge_stat_histories.md).

## Write path

`updateBridgeConfig` in
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go).
Paginates `BridgeApi.GetAllNetworks` and upserts each row; nested
`TokenPairs` go into [`bridge_network_tokens`](bridge_network_tokens.md).

## Read patterns

- **Resolve a network** — `WHERE network_class = $1 AND chain_id = $2`.
- **All known networks** — full scan (tiny table).

## Gotchas

- `metadata` is opaque from the contract — treat it as a black box unless
  you know the network class's format.
- Networks removed from the contract are **not** automatically deleted
  here; they keep their last-known config until manually pruned.
