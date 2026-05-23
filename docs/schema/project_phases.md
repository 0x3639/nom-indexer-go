---
title: project_phases
---

# `project_phases`

## Purpose

Accelerator-Z sub-grants. A project's funding can be broken into multiple
phases; each phase has its own funding ask, status, votes, and `voting_id`.
Refreshed from the project records' inline `Phases` field on the
cached-data sync cadence.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `TEXT` | NO | — | Primary key. Phase hash. |
| `project_id` | `TEXT` | NO | — | Joins to [`projects.id`](projects.md). |
| `voting_id` | `TEXT` | NO | — | ABI-derived ID; phases vote against this, not `id`. |
| `name` | `TEXT` | NO | — | Display name. |
| `description` | `TEXT` | YES | — | Description text. |
| `url` | `TEXT` | YES | — | Reference URL. |
| `znn_funds_needed` | `BIGINT` | NO | — | int64 cap applies. {% include "schema/fragments/int64-cap-caveat.md" %} |
| `qsr_funds_needed` | `BIGINT` | NO | — | Same. |
| `creation_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `accepted_timestamp` | `BIGINT` | NO | — | Unix seconds; 0 if not yet accepted. |
| `status` | `SMALLINT` | NO | — | Phase status enum. |
| `yes_votes` | `SMALLINT` | NO | `0` | |
| `no_votes` | `SMALLINT` | NO | `0` | |
| `total_votes` | `SMALLINT` | NO | `0` | |

## Primary key & indexes

- **Primary key:** `id`.
- `idx_project_phases_project_id`, `idx_project_phases_voting_id`.

## Relations

- `project_id` ↔ [`projects.id`](projects.md).
- `voting_id` ↔ [`votes.voting_id`](votes.md) for phase-level votes.
- `id` ↔ [`votes.phase_id`](votes.md).

## Write path

`ProjectPhaseRepository.Upsert` from `updateCachedData` in
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go).
Walked from each project's inline `Phases` slice. The `voting_id` derivation
is identical to the project-level one (`getVotingID`).

## Read patterns

- **All phases for a project** — `WHERE project_id = $1`.
- **Phase by voting_id** — `WHERE voting_id = $1` (used in vote
  classification when the project-id lookup misses).
- **Phases eligible for a pillar** — `WHERE creation_timestamp >=
  $pillar_spawn_ts`. Used by the voting-activity cron.

## Gotchas

- `accepted_timestamp = 0` is the sentinel for "not yet accepted"; do not
  treat 0 as a real epoch timestamp.
- Same `int16` vote-count cap as [`projects`](projects.md).
- A vote targeting a `voting_id` may match a phase here OR a project — the
  vote-indexing code tries `projects` first, then falls back to this table.
