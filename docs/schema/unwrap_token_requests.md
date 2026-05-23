---
title: unwrap_token_requests
---

# `unwrap_token_requests`

## Purpose

An external-chain → Zenon unwrap intent (a user redeeming a wrapped token
back to the ZTS). Mirror image of [`wrap_token_requests`](wrap_token_requests.md),
keyed on the originating external-chain transaction.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `transaction_hash` | `TEXT` | NO | — | Part of composite PK. External-chain tx hash. |
| `log_index` | `BIGINT` | NO | — | Part of composite PK. External-chain log index. |
| `network_class` | `INT` | NO | — | |
| `chain_id` | `INT` | NO | — | |
| `to_address` | `TEXT` | NO | — | Zenon recipient (`z1…`). |
| `token_standard` | `TEXT` | NO | — | ZTS being received. |
| `token_address` | `TEXT` | NO | — | External-chain token contract that was burned/locked. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `signature` | `TEXT` | NO | — | Orchestrator signature; empty until provided. |
| `registration_momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `redeemed` | `BOOLEAN` | NO | `false` | Whether the recipient has claimed. |
| `revoked` | `BOOLEAN` | NO | `false` | Whether the request was revoked. |
| `redeemable_in` | `BIGINT` | NO | `0` | Delay (in seconds) before claim is allowed. |

## Primary key & indexes

- **Primary key:** `(transaction_hash, log_index)`.
- `idx_unwrap_token_requests_to_address`,
  `idx_unwrap_token_requests_token_standard`,
  `idx_unwrap_token_requests_chain`,
  `idx_unwrap_token_requests_registration_height`,
  `idx_unwrap_token_requests_status` (composite on `redeemed, revoked`).

## Relations

- Same set as [`wrap_token_requests`](wrap_token_requests.md), with
  `to_address` joining to [`accounts.address`](accounts.md) instead of an
  external-chain address.

## Write path

`updateBridgeUnwrapRequests` in
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
on the 1-minute bridge sync. Unlike wraps, unwrap finalization is
**user-initiated and can land out of order** (a user redeems whenever they
want), so the paging strategy differs slightly:

1. Look up the oldest unfinalized (`NOT redeemed AND NOT revoked`) unwrap.
2. Fetch newest-first, upserting each row.
3. Stop once we've reached that floor — but note that older rows may
   already be in DB with stale `redeemed`/`revoked`/`signature` values, so
   the upsert refreshes those columns on every pass.

## Read patterns

- **Unwrap by composite key** — `WHERE transaction_hash = $1 AND log_index = $2`.
- **Pending unwraps** — `WHERE NOT redeemed AND NOT revoked`.
- **Unwraps to a Zenon address** — `WHERE to_address = $1`.
- **Status filter** — uses the composite `(redeemed, revoked)` index.

## Gotchas

- The composite PK matters — a single external-chain transaction can emit
  multiple `Unwrap` log entries, each becoming its own row. Tests against
  `transaction_hash` alone may match multiple unwraps.
- `redeemable_in` is a delay relative to registration, in seconds. To
  compute the actual redeemable time use the joined momentum's timestamp
  plus this column.
- A row with both `redeemed = true` and `revoked = true` should never
  happen — they're mutually exclusive lifecycle terminals. If you see one,
  it indicates a sync error.
- Test-network unwraps can fail with `unknown network` errors during
  bridge sync; this is a known SDK-side issue and surfaces as a warn log
  ("bridge sync: failed to update unwrap requests").
