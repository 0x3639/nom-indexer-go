---
title: bridge_network_tokens
---

# `bridge_network_tokens`

## Purpose

Per-(network, token) bridge configuration: external-chain token contract,
fee, min amount, redeem delay, and the bridgeable/redeemable/owned flags.
One row per (network_class, chain_id, token_standard).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `network_class` | `INT` | NO | — | Composite PK. |
| `chain_id` | `INT` | NO | — | Composite PK. |
| `token_standard` | `TEXT` | NO | — | Composite PK. The ZTS being paired. |
| `token_address` | `TEXT` | NO | — | External-chain token contract. |
| `bridgeable` | `BOOLEAN` | NO | `false` | Whether Zenon → external wraps are allowed for this pair. |
| `redeemable` | `BOOLEAN` | NO | `false` | Whether external → Zenon unwraps are allowed. |
| `owned` | `BOOLEAN` | NO | `false` | Whether the bridge contract owns the external-chain token. |
| `min_amount` | `BIGINT` | NO | `0` | Smallest amount that can be bridged. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `fee_percentage` | `INT` | NO | `0` | Bridge fee, basis points × 10. |
| `redeem_delay` | `INT` | NO | `0` | Seconds before an unwrap can be claimed on Zenon side. |
| `metadata` | `TEXT` | NO | `''` | Opaque per-pair metadata. |

## Primary key & indexes

- **Primary key:** `(network_class, chain_id, token_standard)`.

## Relations

- `(network_class, chain_id)` ↔ [`bridge_networks`](bridge_networks.md).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).

## Write path

`updateBridgeConfig` — nested under the network loop. Each network's
`TokenPairs` slice produces one row per token here.

## Read patterns

- **All token pairs for a network** — `WHERE network_class = $1 AND
  chain_id = $2`.
- **All bridges for a token** — `WHERE token_standard = $1`. Combine
  with `bridgeable` / `redeemable` filters as needed.

## Gotchas

- Removed pairs are not deleted automatically (same as
  [`bridge_networks`](bridge_networks.md)).
- `fee_percentage` is **not** a plain percent — it's the contract's
  `feePercentage` field with the units the contract uses (basis-points-ish).
  Refer to the bridge contract spec for the exact divisor.
