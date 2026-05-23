---
title: votes
---

# `votes`

## Purpose

Pillar votes on Accelerator-Z projects and phases. One row per
(voter, voting_id) pair — a voter who re-votes overwrites their previous
vote rather than appending. The voting_id ties back to either
[`projects.voting_id`](projects.md) or
[`project_phases.voting_id`](project_phases.md).

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | `SERIAL` | NO | — | Primary key. |
| `momentum_hash` | `TEXT` | NO | — | Joins to [`momentums.hash`](momentums.md). |
| `momentum_timestamp` | `BIGINT` | NO | — | Unix seconds. |
| `momentum_height` | `BIGINT` | NO | — | Joins to [`momentums.height`](momentums.md). |
| `voter_address` | `TEXT` | NO | — | The voting pillar's owner address. |
| `project_id` | `TEXT` | NO | — | Empty string if the voter targeted a phase only. |
| `phase_id` | `TEXT` | NO | `''` | Set when the vote targets a phase. |
| `voting_id` | `TEXT` | NO | — | The ABI-derived voting ID; matches either project or phase. |
| `vote` | `SMALLINT` | NO | — | 0=yes, 1=no, 2=abstain (per the contract). |

## Primary key & indexes

- **Primary key:** `id`.
- `uq_votes_voter_voting` — UNIQUE on `(voter_address, voting_id)`. Added
  in migration 006 to enforce one-vote-per-voter-per-proposal.
- `idx_votes_voter_address`, `idx_votes_project_id`, `idx_votes_phase_id`,
  `idx_votes_voting_id`.

## Relations

- `voter_address` ↔ [`pillars.owner_address`](pillars.md) (pillar that voted).
- `project_id` ↔ [`projects.id`](projects.md).
- `phase_id` ↔ [`project_phases.id`](project_phases.md).
- `voting_id` ↔ `projects.voting_id` OR `project_phases.voting_id`.

## Write path

`indexAcceleratorContract` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go),
on `VoteByName` or `VoteByProdAddress`:

1. The decoded inputs supply `id` (the voting_id) and `vote` value.
2. The repository tries `ProjectRepository.GetIDFromVotingID(id)` first;
   if it misses, falls back to
   `ProjectPhaseRepository.GetProjectAndPhaseIDFromVotingID(id)`.
3. For `VoteByName`, the voter is the pillar owner resolved from the
   `name` input through the cached pillar map. For `VoteByProdAddress`,
   the voter is the paired send block's address directly.
4. `VoteRepository.InsertBatch` upserts on `(voter_address, voting_id)`,
   replacing the prior vote.

## Read patterns

- **Latest vote for (pillar, proposal)** — direct unique lookup on the
  composite.
- **All votes by a pillar** — `WHERE voter_address = $1 ORDER BY
  momentum_height DESC`.
- **Vote counts for a project** — `WHERE project_id = $1` (groups by vote
  value). Used by `getVoteCountForProjects` in the voting-activity cron.
- **Vote counts for a phase set** — `WHERE phase_id = ANY($1)`.

## Gotchas

- `phase_id = ''` (empty string) is the sentinel for "project-level vote",
  not NULL. Tests like `phase_id IS NULL` will never match.
- A pillar can have at most one row per `voting_id` (enforced by the
  unique constraint). The re-vote behavior is overwrite, not append; if
  you need vote history, walk `account_blocks WHERE method LIKE
  'VoteBy%' AND address = $1` directly.
- `vote` values 0/1/2 are unsigned in spirit; SMALLINT accepts a wider
  range but the contract only emits those three.
- Pre-migration-006 duplicates were collapsed by the migration. Any
  external mirror must be re-synced after that point.
