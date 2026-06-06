---
title: bridge_time_challenges
---

# `bridge_time_challenges`

## Purpose

Pending **time challenges** for security-sensitive bridge methods. A time
challenge is the mandatory delay window that must elapse before a sensitive
bridge action takes effect.

These methods use a **two-call mechanism**:

1. **First call** registers the challenge — it records the call's parameters
   (as `params_hash`) and the momentum height at which the window opened
   (`challenge_start_height`). Nothing changes on-chain yet.
2. **Second call**, issued only after the delay has elapsed, actually
   executes the action — but only if its parameters hash to the same
   `params_hash`. A second call with different parameters does not execute;
   it starts (or restarts) a fresh challenge.

The window blocks an attacker who compromised an admin key from instantly
pushing through a malicious change: guardians and observers have at least
`delay` momentums to notice and react before the action can land.

One row per **currently-pending** challenge, keyed by `method_name`. Because
each challenged method can have at most one open challenge at a time, the
method name is a natural primary key. The table holds only challenges that are
still pending: a challenge disappears once it is executed or expires (see
**Notes** — the table is pruned to an authoritative set, so an empty table is
the normal state).

## Columns

All 6 columns from
[`migrations/016_bridge_time_challenges.up.sql`](https://github.com/0x3639/nom-indexer-go/blob/main/migrations/016_bridge_time_challenges.up.sql).
Heights and timestamps are int64 `BIGINT`; hashes follow the
[schema conventions](conventions.md).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `method_name` | `TEXT` | NO | — | Primary key. The challenged bridge method (e.g. `SetTokenPair`). See the method/delay table below. |
| `params_hash` | `TEXT` | NO | `''` | Hash of the pending call's parameters. The second call must hash to this value to execute. |
| `challenge_start_height` | `BIGINT` | NO | `0` | Momentum height at which the challenge was registered (the window opened). |
| `delay` | `BIGINT` | NO | `0` | Length of the delay window in **momentums**, taken from bridge SecurityInfo. See **Notes** for the `delay = 0` transient case. |
| `end_height` | `BIGINT` | NO | `0` | Earliest momentum height at which the action may execute: `challenge_start_height + delay`. |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds of the last bridge sync that observed this challenge. |

## Challenged methods and their delay

The four security-sensitive bridge methods are governed by one of two delays
from [`bridge_security_info`](bridge_security_info.md):

| Method | Governing delay | What it does |
|---|---|---|
| `SetTokenPair` | **SoftDelay** | Add/update a wrap-token pair for a bridge network. |
| `ChangeTssECDSAPubKey` | **SoftDelay** | Rotate the TSS ECDSA public key used to sign bridge operations. |
| `ChangeAdministrator` | **AdministratorDelay** | Transfer the bridge administrator role. |
| `NominateGuardians` | **AdministratorDelay** | Replace the bridge guardian set. |

The indexer applies this mapping while writing each row: methods default to
`SoftDelay`, with `ChangeAdministrator` and `NominateGuardians` switched to
`AdministratorDelay`. `delay` is then this value and `end_height` is
`challenge_start_height + delay`.

## Primary key & indexes

- **Primary key:** `method_name`.

## Relations

- Delays are sourced from [`bridge_security_info`](bridge_security_info.md)
  (`soft_delay`, `administrator_delay`).
- `ChangeAdministrator` corresponds to the administrator tracked in
  [`bridge_admin`](bridge_admin.md); `NominateGuardians` to the set in
  [`bridge_guardians`](bridge_guardians.md).

## Write path

All writes come from
[`updateBridgeConfig`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go)
on the bridge sync loop (~1-minute tick). It is RPC-polled, not block-driven:

1. The loop calls `BridgeApi.GetTimeChallengesInfo()` to fetch the full list
   of currently-pending challenges.
2. For each challenge it computes `delay` from the method→delay mapping above
   (using the SecurityInfo fetched earlier in the same tick), then **upserts**
   the row via `UpsertTimeChallenge` with
   `end_height = challenge_start_height + delay` and
   `last_updated_timestamp = now`.
3. It collects the method names just seen and calls
   `DeleteTimeChallengesNotIn(keep)` to **prune** every row not in the latest
   list — that is how an executed or expired challenge is removed.

## Read patterns

- **All pending challenges** — `SELECT * FROM bridge_time_challenges`.
- **Is a specific method challenged right now** — direct PK lookup on
  `method_name`.
- **Challenges ready to execute** — `WHERE end_height <= <current_height>`
  (the delay has elapsed).

## Notes

### Authoritative-set / prune semantics

The table is **not** an append-only history. Each sync rewrites it to match
exactly what `GetTimeChallengesInfo` returns: rows seen are upserted, rows no
longer returned are deleted. A row therefore exists if and only if the
corresponding challenge is currently pending. For history of executed admin
changes, consult the relevant per-table pages
([`bridge_admin`](bridge_admin.md), [`bridge_guardians`](bridge_guardians.md))
or the on-chain account blocks — not this table.

### An empty table is normal

Pending time challenges are rare; most of the time no challenge is open and
the table is empty. An empty result means "no security-sensitive bridge
action is currently in its delay window," not an indexer failure.

### `delay = 0` transient case

`delay` and `end_height` are derived from SecurityInfo fetched at the start of
the same sync tick. If that SecurityInfo call fails transiently, the captured
`softDelay`/`adminDelay` are `0`, so a challenge inserted on that tick gets
`delay = 0` and `end_height = challenge_start_height`. This self-corrects: the
next tick (with SecurityInfo available) upserts the same row with the correct
`delay` and `end_height`. Treat a `delay = 0` row as provisional.
