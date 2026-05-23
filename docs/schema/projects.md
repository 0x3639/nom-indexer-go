---
title: projects
---

# `projects`

## Purpose

Accelerator-Z funding proposals. Refreshed from
`AcceleratorApi.GetAll` on the cached-data sync cadence (5 minutes). Each
project carries a `voting_id` derived by ABI-encoding `VoteByName(id, "", 0)`
through the Accelerator ABI — pillars vote on this derived ID, not the
project ID directly.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. Project hash. |
| `voting_id` | `TEXT` | NO | — | ABI-derived ID that votes reference. Indexed. |
| `owner` | `TEXT` | NO | — | Project owner address. |
| `name` | `TEXT` | NO | — | Display name. |
| `description` | `TEXT` | YES | — | Description text. |
| `url` | `TEXT` | YES | — | Reference URL. |
| `znn_funds_needed` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `qsr_funds_needed` | `BIGINT` | NO | — | Same. |
| `creation_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `last_update_timestamp` | `BIGINT` | NO | — | Last status/vote refresh. |
| `status` | `SMALLINT` | NO | — | Voting/lifecycle status enum from the contract. |
| `yes_votes` | `SMALLINT` | NO | `0` | Aggregate yes count. |
| `no_votes` | `SMALLINT` | NO | `0` | Aggregate no count. |
| `total_votes` | `SMALLINT` | NO | `0` | Aggregate total. |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_projects_owner`, `idx_projects_voting_id`.

## Relations

- `id` ↔ [`project_phases.project_id`](project_phases.md),
  [`votes.project_id`](votes.md).
- `voting_id` ↔ [`votes.voting_id`](votes.md) for project-level votes.
- `owner` ↔ [`accounts.address`](accounts.md).

## Write path

`ProjectRepository.Upsert` from `updateCachedData` in
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
called for every project returned by the paginated `AcceleratorApi.GetAll`.
`voting_id` is computed once via the `getVotingID` helper (encode/decode
round-trip on the Accelerator ABI).

The 5-minute cadence means newly-created projects appear after a short
delay, and the vote counts are eventually-consistent (the source of truth
is the contract; this table mirrors the latest view).

## Read patterns

- **Project detail** — direct PK lookup.
- **All open projects** — `WHERE status = <open>`.
- **Lookup by voting_id** — `WHERE voting_id = $1`. Used by vote
  classification to map a `VoteByName` arg back to its project.
- **Pillar's eligible projects** — `WHERE creation_timestamp >=
  $pillar_spawn_ts`. Used by the voting-activity cron.

## Gotchas

- `description` and `url` are user-supplied strings — treat as untrusted
  when rendering.
- The `status` enum values are not documented in this repo's schema; see
  the Zenon contract specs.
- `voting_id` is **not** the same as `id`. Phases have their own voting
  IDs too (in [`project_phases`](project_phases.md)). A `votes.voting_id`
  query must check both tables.
- `int16` vote count columns cap at 32,767. Today's pillar count is ~28,
  so this is plenty of headroom; reconsider if pillar count ever exceeds
  10,000.
