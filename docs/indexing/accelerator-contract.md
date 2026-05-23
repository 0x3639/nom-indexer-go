---
title: Accelerator-Z contract
---

# Accelerator-Z contract

## Contract address

`z1qxemdeddedxaccelerat0rxxxxxxxxxxp4tk22` — `AcceleratorAddress`.

## Methods observed

Handler: `indexAcceleratorContract` in `embedded.go`.

| Method | Inputs | Triggers |
|---|---|---|
| `VoteByName` | `id` (voting_id), `name` (pillar name), `vote` | Insert into [`votes`](../schema/votes.md). |
| `VoteByProdAddress` | `id` (voting_id), `vote` | Insert into [`votes`](../schema/votes.md). |
| `CreateProject` | (debug-logged only) | Project rows arrive via `updateCachedData`, not per-block. |
| `AddPhase`, `UpdatePhase` | (debug-logged only) | Phase rows arrive via `updateCachedData`. |

Project + phase records and their vote totals refresh from
`AcceleratorApi.GetAll` on the cached-data sync cadence (5 minutes).
Only individual votes are indexed in real time.

## Per-method write effects

- **VoteByName**
    - Resolves the voter via `getPillarOwnerAddress(name)` from the
      cached pillar map; falls back to the paired send block's address.
    - Resolves the proposal: tries
      `ProjectRepository.GetIDFromVotingID(id)` first; if that misses,
      `ProjectPhaseRepository.GetProjectAndPhaseIDFromVotingID(id)` for
      the (project_id, phase_id) pair.
    - `votes`: `InsertBatch` upserts on `(voter_address, voting_id)` per
      migration 006's unique constraint.
- **VoteByProdAddress**
    - Same resolution path but the voter is the paired send block's
      address directly (no name lookup).

## Special computation

- **`voting_id`** is the ABI-encoded `VoteByName(id, "", 0)` parameter
  list's first decoded value. Both projects and phases derive their
  voting IDs this way; consumers vote against the voting ID, not the
  raw `id`. The helper is `getVotingID(id)` in
  [`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go).

## Tests

- [`internal/indexer/voting_id_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/voting_id_test.go) — roundtrip and pass-through-on-invalid behavior.
- [`internal/indexer/decoder_real_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_real_test.go) — `VoteByName` end-to-end decode through the SDK ABI.
- [`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go) — `TestIntegration_Vote_UpsertDedupesPerVoterAndVotingID` verifies the dedup behavior post-migration-006.

## Notes

`votes.phase_id = ''` (empty string) — not NULL — when the vote targets a
project directly rather than a phase. See
[`schema/votes.md`](../schema/votes.md).
