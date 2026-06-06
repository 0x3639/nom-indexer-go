---
title: swap_assets
---

# `swap_assets`

## Purpose

A snapshot of the **remaining unswapped legacy genesis balances** still held by
the embedded **Swap** contract, keyed by `keyIdHash` (the contract's storage
key for each genesis allocation). Where [`swap_retrievals`](swap_retrievals.md)
is the per-claim event log, this table is the *current state*: how much ZNN and
QSR each genesis key still has available to claim.

Rows are refreshed (upserted) on the indexer's cached-data refresh from the
`SwapApi.GetAssets()` RPC, so the table reflects the authoritative remaining
balances as of the last successful snapshot.

## Columns

All 4 columns from
[`migrations/015_swap.up.sql`](https://github.com/0x3639/nom-indexer-go/blob/main/migrations/015_swap.up.sql).
Amounts are int64 `BIGINT`; timestamps are Unix seconds; hashes follow the
[schema conventions](conventions.md).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `key_id_hash` | `TEXT` | NO | — | Primary key. 64-char hex storage key (`keyIdHash`) for the genesis allocation. |
| `znn` | `BIGINT` | NO | `0` | Remaining unclaimed ZNN for this key. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `qsr` | `BIGINT` | NO | `0` | Remaining unclaimed QSR for this key. int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds of the snapshot that last wrote this row (`time.Now()` at sync). |

## Primary key & indexes

- **Primary key:** `key_id_hash`.

## Relations

- `key_id_hash` is the swap contract's internal storage key, not an
  account-block hash. It is not directly joinable to
  [`swap_retrievals`](swap_retrievals.md), which is keyed by claim send-block.

## Write path

- **`UpsertAsset`** from
  [`syncSwapAssets`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
  called from `updateCachedData` on the cached-data refresh cadence (not in the
  per-momentum transaction). It calls `SwapApi.GetAssets()`, which returns
  `map[types.Hash]*embedded.SwapAssetEntrySimple` keyed by `keyIdHash`, and
  upserts one row per key with the current `Znn`/`Qsr` remaining balances and
  `last_updated_timestamp = time.Now()`. RPC and per-row upsert failures are
  logged and skipped so a swap error never aborts the cached-data refresh.

## Read patterns

- **Remaining balance for a key** — direct PK lookup on `key_id_hash`.
- **Total unswapped supply** — `SELECT sum(znn), sum(qsr) FROM swap_assets`.

## Notes

### Known limitation — this table is currently EMPTY

As of `znn-sdk-go@v0.1.12`, **`swap_assets` will remain empty** even on a
fully synced indexer. This is **not** an indexer bug — it is an upstream SDK
defect:

- `SwapApi.GetAssets()` in `znn-sdk-go@v0.1.12` passes a **nil result map by
  value** to the RPC client. Because the map is passed by value (and is nil),
  the decoded response is discarded and the method returns an **empty map**
  with no error.
- `syncSwapAssets` therefore iterates over zero entries, writes no rows, and
  logs `swap sync: assets snapshot complete` with `count=0`. No error is
  raised; the cached-data refresh completes normally.

The table will start populating once one of the following happens:

1. The upstream SDK is fixed to allocate the result map before the RPC call, or
2. A local wrapper is added in this repo that allocates the map and calls the
   RPC directly (bypassing the buggy SDK method).

**Operators and consumers should not be alarmed by an empty `swap_assets`
table** — it reflects the SDK limitation above, not missing or failed
indexing. The companion [`swap_retrievals`](swap_retrievals.md) table is
**unaffected**, because it is built from account-block (`RetrieveAssets`) data
rather than this RPC, and populates normally.
