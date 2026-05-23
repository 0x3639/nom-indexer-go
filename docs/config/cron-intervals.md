---
title: Cron intervals
---

# Cron intervals

The indexer runs five periodic loops. Two are tunable via config; three
are hardcoded. This page documents the tradeoffs of each and when to
deviate from defaults.

| Loop | Default | Tunable | Source |
|---|---|---|---|
| Bridge sync (wrap/unwrap + config) | 1 min | No | `runBridgeSyncLoop` |
| Cached data sync (pillars, sentinels, projects) | 5 min | No | `runCachedDataSyncLoop` |
| Voting activity refresh | 10 min | `cron.voting_activity_interval` | `runVotingActivity` |
| Token holder count refresh | 10 min | `cron.token_holders_interval` | `runTokenHolderCounts` |
| Daily stat snapshots | 1 h | No | `runStatSnapshots` |

All are in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go)
or
[`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go).

## Bridge sync — 1 minute

Drives [`wrap_token_requests`](../schema/wrap_token_requests.md),
[`unwrap_token_requests`](../schema/unwrap_token_requests.md), and the
six bridge-config tables. A faster cadence would reduce wrap/unwrap
latency for explorer queries; the cost is one RPC round-trip per minute
to a node that may be slow.

Hardcoded because the node-side rate limits and the user-visible latency
target make tuning a one-line change in code, not config.

## Cached data sync — 5 minutes

Drives [`pillars`](../schema/pillars.md),
[`sentinels`](../schema/sentinels.md),
[`projects`](../schema/projects.md), and
[`project_phases`](../schema/project_phases.md). Each call walks
paginated APIs that can take ~100 seconds for the accelerator alone
(see [`docs/operations/failure-modes.md`](../operations/failure-modes.md)).

Hardcoded because the accelerator pagination is the bottleneck — going
faster would not help and could starve the bridge sync.

## Voting activity — 10 min

Tunable: `cron.voting_activity_interval = "10m"`.

Recomputes `pillars.voting_activity` for every pillar by counting
distinct proposals they've voted on out of proposals eligible since
spawn. Cheap (DB-only queries), but the value updates infrequently —
votes don't change minute-to-minute. 10 minutes is conservative.

Drop to `1m` if you're testing voting flows and want near-real-time
visibility; raise to `1h` on a quiet network to save query load.

## Token holder counts — 10 min

Tunable: `cron.token_holders_interval = "10m"`.

For each [`tokens`](../schema/tokens.md) row, runs
`SELECT COUNT(*) FROM balances WHERE token_standard = $1 AND balance > 0`
(uses the partial index, cheap). 328 tokens × 1 count ≈ 200ms total.

Drop to `1m` on a chain with active token issuance; raise to `1h` on a
quiet chain.

## Stat snapshots — 1 hour

Drives the four `*_stat_histories` tables. Each tick runs a handful of
aggregation queries and writes one row per (date, key). Idempotent
upserts — running mid-day rewrites the day's row.

Hardcoded because the per-day granularity caps the useful resolution. A
faster cadence keeps the current day's row fresher but doesn't change
historical data.

## What's *not* a cron loop

- **Momentum sync** — driven by `SubscriberApi.ToMomentums`, plus a
  catch-up pass before each reconnect. See
  [`docs/architecture/sync-and-recovery.md`](../architecture/sync-and-recovery.md).
- **Backfill** — a one-shot pass when `BACKFILL_ON_STARTUP=true` or
  when `cmd/backfill` is run manually.
