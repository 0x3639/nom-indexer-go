---
title: account_blocks
---

# `account_blocks`

## Purpose

Every transaction the indexer has seen, with decoded ABI method + inputs
when the transaction targets an embedded contract. One row per blockchain
account block; the table is the workhorse for every "what happened" query.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `hash` | `TEXT` | NO | — | Primary key. 64-char hex. |
| `momentum_hash` | `TEXT` | YES | — | Joins to [`momentums.hash`](momentums.md). |
| `momentum_timestamp` | `BIGINT` | YES | — | Unix seconds, denormalized for date-bucketed queries. |
| `momentum_height` | `BIGINT` | YES | — | Joins to [`momentums.height`](momentums.md). |
| `block_type` | `SMALLINT` | NO | — | SDK `BlockType*` enum: 1=GenesisReceive, 2=UserSend, 3=UserReceive, 4=ContractSend, 5=ContractReceive. |
| `height` | `BIGINT` | NO | — | Per-account block height (each address has its own ladder). |
| `address` | `TEXT` | NO | — | Sender (`z1…`). |
| `to_address` | `TEXT` | YES | — | Recipient (`z1…`). Empty for some embedded contract sends. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `token_standard` | `TEXT` | YES | — | Empty token standard `zts1qqqqq…587y` means "no transfer". |
| `data` | `TEXT` | YES | — | Hex-encoded raw call data. |
| `method` | `TEXT` | YES | `''` | Decoded ABI method name when targeting an embedded contract. |
| `input` | `JSONB` | YES | `'{}'` | Decoded ABI inputs as a flat `{name: stringified-value}` map. |
| `paired_account_block` | `TEXT` | YES | `''` | The send/receive counterpart's hash. |
| `descendant_of` | `TEXT` | YES | `''` | Parent block hash when this block was emitted as a child (used by reward backfill). |

## Primary key & indexes

- **Primary key:** `hash`.
- `idx_account_blocks_address`, `idx_account_blocks_to_address`,
  `idx_account_blocks_momentum_height`, `idx_account_blocks_token_standard`,
  `idx_account_blocks_method`.

## Relations

- `momentum_hash` / `momentum_height` ↔ [`momentums`](momentums.md).
- `address`, `to_address` ↔ [`accounts.address`](accounts.md) (sender / recipient).
- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `paired_account_block` is a self-reference within `account_blocks`.
- Used as the source for [`token_mints`](token_mints.md), [`token_burns`](token_burns.md),
  [`reward_transactions`](reward_transactions.md), and the per-contract event
  tables ([`pillar_updates`](pillar_updates.md), [`votes`](votes.md), etc.).

## Write path

- Inserted by
  [`AccountBlockRepository.InsertBatch`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account_block.go)
  from `processAccountBlocks` in
  [`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go).
- `paired_account_block` is set both on the initial insert and via a separate
  `UPDATE` (`UpdatePairedBlockBatch`) so paired-block resolution doesn't have
  to wait for the counterpart row.
- `descendant_of` is set by `UpdateDescendantOfBatch` when the indexer sees
  a parent → child relationship in `block.DescendantBlocks`.

The decoded `method` + `input` come from `tryDecodeTxData` in
[`internal/indexer/decoder.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder.go).
NUL bytes are scrubbed before insert (see
[`sanitizeJSONForPostgres`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account_block.go)
— PG rejects `\u0000` in JSONB).

## Read patterns

- **Transaction by hash** — direct PK lookup.
- **Account history** — `WHERE address = $1 OR to_address = $1 ORDER BY momentum_height DESC`.
- **Method-filtered scan** — `WHERE method = 'VoteByName'` (uses the method
  index).
- **Reward-receive detection** — see
  [`indexing/rewards.md`](../indexing/rewards.md) for the canonical pattern.

## Gotchas

- The empty token standard `zts1qqqqqqqqqqqqqqqqtq587y` is **not** a
  placeholder — it means the block carries no token transfer (e.g., a
  receive of a contract-emitted side effect). Treat it as a sentinel, not
  a real ZTS.
- `block_type` literals are easy to get wrong. Use the SDK constants
  ([`utils.BlockTypeUserReceive`](https://github.com/0x3639/znn-sdk-go) etc.);
  see [`docs/reference/known-issues.md`](../reference/known-issues.md) for
  the historical bug.
- `input` may contain user-controlled strings. Treat it as untrusted when
  rendering in any UI; sanitization is only against PG's JSONB requirements,
  not against XSS.
- The indexer reprocesses pre-existing rows via `ON CONFLICT (hash) DO
  UPDATE SET method, input, paired_account_block` — re-running the
  `repair-votes` script will correctly rewrite stale decoded fields.
