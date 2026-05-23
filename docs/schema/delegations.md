---
title: delegations
---

# `delegations`

## Purpose

Time-bucketed delegation history. Each row represents one continuous
interval of "delegator X delegated to pillar P" — `started_at` is set on
`Delegate`, `ended_at` is set on a subsequent `Delegate` (re-target) or
`Undelegate`. Active (open) intervals have `ended_at IS NULL`.

Introduced in migration `011` to close a gap against the zenonhub PHP
indexer (we previously kept only the current delegation in
[`accounts.delegate`](accounts.md)).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `BIGSERIAL` | NO | — | Primary key. |
| `delegator_address` | `TEXT` | NO | — | Account that delegated. |
| `pillar_owner_address` | `TEXT` | NO | — | Pillar the delegation went to. |
| `started_at` | `BIGINT` | NO | — | Unix seconds. |
| `ended_at` | `BIGINT` | YES | — | Unix seconds when this delegation ended; NULL for the currently-active row. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_delegations_delegator` on `delegator_address`.
- `idx_delegations_pillar` on `pillar_owner_address`.
- `idx_delegations_open` — partial index on `delegator_address`
  `WHERE ended_at IS NULL`, optimized for "what is X currently delegated to?".

## Relations

- `delegator_address` ↔ [`accounts.address`](accounts.md).
- `pillar_owner_address` ↔ [`pillars.owner_address`](pillars.md).

## Write path

`indexPillarContract` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go):

- On `Delegate`: `DelegationRepository.CloseActiveBatch` closes the prior
  open row (if any) by setting `ended_at = momentum_timestamp`, then
  `OpenBatch` opens a new interval pointing at the new pillar.
- On `Undelegate`: `CloseActiveBatch` only — no new interval opens.

Both writes are part of the same momentum's transactional batch, so a
delegator can never have two open intervals.

## Read patterns

- **Current delegation** — `WHERE delegator_address = $1 AND ended_at IS
  NULL` (partial-index lookup).
- **Active delegator count for a pillar** — `WHERE pillar_owner_address =
  $1 AND ended_at IS NULL`. Used by `snapshotPillarStats` to fill
  `pillar_stat_histories.total_delegators`.
- **Delegation history for an account** — `WHERE delegator_address = $1
  ORDER BY started_at DESC`.
- **Pillar membership at time T** — `WHERE pillar_owner_address = $1 AND
  started_at <= T AND (ended_at IS NULL OR ended_at > T)`. Used for
  historical delegate-reward attribution.

## Gotchas

- A row with `started_at = ended_at` is valid — it means the delegator
  switched pillars in the same momentum (rare but legal).
- The partial unique-ish invariant (one open row per delegator) is enforced
  by the indexer's transactional close-then-open ordering, **not** by a DB
  constraint. If you write directly to the table from a backfill script,
  preserve that ordering.
- Pre-011 history is not in this table. Backfilling requires walking
  `account_blocks WHERE method IN ('Delegate', 'Undelegate')` in order and
  replaying the close/open logic; there is no existing script today.
- `accounts.delegate` is the **current** pillar; consult that for fast
  lookups, this table for historical interval queries.
