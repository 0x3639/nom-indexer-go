---
title: pillar_updates
---

# `pillar_updates`

## Purpose

Append-only log of pillar configuration changes. Each `Register`,
`RegisterLegacy`, or `UpdatePillar` event becomes a row. Lets consumers
reconstruct the pillar's withdraw/producer history (e.g., "who was this
producer at height H?") without time-travelling through `account_blocks`.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `SERIAL` | NO | — | Primary key. Autoincrement. |
| `name` | `TEXT` | NO | — | Pillar name at the time of the update. |
| `owner_address` | `TEXT` | NO | — | Pillar's owner. |
| `producer_address` | `TEXT` | NO | — | Block-production address at this revision. |
| `withdraw_address` | `TEXT` | NO | — | Reward-destination address at this revision. |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds of the block that emitted the update. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `momentum_hash` | `TEXT` | NO | — | Joins to [`momentums.hash`](momentums.md). |
| `give_momentum_reward_percentage` | `SMALLINT` | NO | — | Reserved by the schema; current handler leaves it at `0`. |
| `give_delegate_reward_percentage` | `SMALLINT` | NO | — | Reserved by the schema; current handler leaves it at `0`. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_pillar_updates_owner_address`, `idx_pillar_updates_producer_address`,
  `idx_pillar_updates_withdraw_address`,
  `idx_pillar_updates_momentum_height`.

## Relations

- `owner_address` ↔ [`pillars.owner_address`](pillars.md).
- `producer_address` is used to resolve `momentums.producer_owner` /
  `producer_name` for historical blocks (see
  `PillarUpdateRepository.GetInfoAtHeightByProducer`).
- `withdraw_address` participates in reward classification via
  `PillarRepository.IsWithdrawAddress` (checks both current pillars and
  this history).

## Write path

[`PillarUpdateRepository.InsertBatch`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/pillar_update.go)
from `indexPillarContract` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go):

- `Register` / `RegisterLegacy`: writes a row using inputs `name`,
  `producerAddress`, `rewardAddress` (the withdraw destination) and the
  paired send block's address as the owner.
- `UpdatePillar`: same shape, owner pulled from the `pillarOwner` input
  enriched by `processAccountBlocks`.

Reward-percentage fields are not populated by the current
`indexPillarContract` handler, so they are written as `0`.

## Read patterns

- **Pillar history** — `WHERE owner_address = $1 ORDER BY id DESC`.
- **Producer → pillar resolution at height** —
  `WHERE producer_address = $1 AND momentum_height <= $H ORDER BY id DESC
  LIMIT 1`. Used by `getPillarInfoForProducer`.
- **All historical withdraw addresses** — `SELECT DISTINCT withdraw_address`.

## Gotchas

- This is append-only; there is no `UPDATE` or `DELETE` flow. To revoke a
  pillar, see [`pillars.is_revoked`](pillars.md). Revocation does **not**
  create a row here — only Register/Update calls do.
- Reward-percentage columns currently remain `0` even when the live
  pillar row has non-zero splits from the cached Pillar API refresh.
- The `id` SERIAL is the natural ordering key — momentum_height ties may
  not preserve emit order within the same block. Use `id` as the
  tie-breaker if reconstructing precise sequence.
