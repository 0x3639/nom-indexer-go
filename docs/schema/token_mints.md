---
title: token_mints
---

# `token_mints`

## Purpose

One row per `Token.Mint` event. The issuer (the embedded reward contract
or token owner that called `Mint`), the receiver, and the amount are
captured per-event so consumers can derive token issuance history,
reward attribution, and inflation curves without joining back through
`account_blocks`.

This table was introduced in migration `007` to close a gap against the
zenonhub PHP indexer (we previously only had a `total_burned` counter and
no per-event mint history).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `BIGSERIAL` | NO | — | Primary key. Autoincrement. |
| `account_block_hash` | `TEXT` | NO | — | The contract-receive block on the Token contract. Unique. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `token_standard` | `TEXT` | NO | — | The token being minted. |
| `issuer` | `TEXT` | NO | — | The address that called `Mint` on the Token contract — typically an embedded contract (pillar, sentinel, stake, liquidity, bridge) or a token owner. |
| `receiver` | `TEXT` | NO | — | The address credited with the minted amount. |
| `amount` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |

## Primary key & indexes

- **Primary key:** `id`.
- `uq_token_mints_block_hash` — unique index on `account_block_hash`,
  ensuring one mint event per source block (idempotent re-inserts).
- `idx_token_mints_token_standard`, `idx_token_mints_issuer`,
  `idx_token_mints_receiver`, `idx_token_mints_momentum_height`.

## Relations

- `token_standard` ↔ [`tokens.token_standard`](tokens.md).
- `issuer`, `receiver` ↔ [`accounts.address`](accounts.md).
- `account_block_hash` ↔ [`account_blocks.hash`](account_blocks.md).
- `momentum_height` ↔ [`momentums.height`](momentums.md).

## Write path

`indexTokenContract` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go)
calls `InsertMintBatch` whenever it sees a `Mint` contract-receive on the
Token contract. The decoded inputs supply `tokenStandard`, `amount`,
`receiveAddress`; the issuer is the paired send block's address.

The historical backfill script
[`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards)
populates this table from pre-007 history when run against an
already-indexed DB.

## Read patterns

- **Mints per token** — `WHERE token_standard = $1 ORDER BY momentum_height
  DESC LIMIT N`.
- **Mints by issuer** (reward attribution) — `WHERE issuer = $1`. Joining
  on `issuer IN (PillarAddress, SentinelAddress, StakeAddress)` produces a
  per-contract reward stream.
- **Mints to a receiver** — `WHERE receiver = $1`. Cross-checks against
  [`reward_transactions`](reward_transactions.md) for the same address.
- **Daily mint totals** — used by `snapshotTokenStats` to fill
  `token_stat_histories.daily_minted`.

## Gotchas

- The `issuer` is the **direct** caller of `Mint`. For reward mints this is
  the embedded reward contract; for owner-initiated mints it's the token
  owner. Pillar/delegate reward classification needs an additional lookup
  through [`pillar_updates.withdraw_address`](pillar_updates.md) — see
  [`indexing/rewards.md`](../indexing/rewards.md).
- Pre-migration-007 history is **not** in this table by default; run the
  backfill-rewards script if you need it.
- `account_block_hash` uniqueness means re-running the indexer over the
  same height is safe — duplicate inserts are dropped by the unique index.
