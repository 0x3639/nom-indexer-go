---
title: bridge_guardians
---

# `bridge_guardians`

## Purpose

Active bridge guardians — the set of addresses that can vote on bridge
administrative actions. Refreshed from `BridgeApi.GetSecurityInfo`. Each
guardian is one row keyed by address.

## Columns

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `address` | `TEXT` | NO | — | Primary key. Guardian address. |
| `nominated` | `BOOLEAN` | NO | `false` | True when the address is in the current guardian set. |
| `accepted` | `BOOLEAN` | NO | `false` | True when the guardian has cast a vote. |
| `last_updated_timestamp` | `BIGINT` | NO | `0` | Unix seconds of last refresh. |

## Primary key & indexes

- **Primary key:** `address`.

## Relations

- `address` ↔ [`accounts.address`](accounts.md).

## Write path

`updateBridgeConfig`:

1. `GetSecurityInfo` returns nominated + accepted guardian lists.
2. Each address in either list is upserted with the appropriate flags and
   `last_updated_timestamp = now`.
3. `MarkGuardiansAbsent(now)` flips every row whose `last_updated_timestamp`
   is older than `now` to `nominated = false AND accepted = false` — that's
   how a removed guardian is recorded.

## Read patterns

- **Active guardians** — `WHERE nominated = true OR accepted = true`.
- **Historical guardian** — direct PK lookup; check the flags to see
  current status.

## Gotchas

- Rows are never deleted, only flipped to inactive. A previously-known
  guardian who was removed will still be here, just with both flags false.
- The "absent" sweep happens once per bridge sync — if a sync fails
  midway, an active guardian could briefly look removed. Re-run by waiting
  for the next 1-minute tick.
