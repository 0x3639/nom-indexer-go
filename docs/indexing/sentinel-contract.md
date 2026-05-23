---
title: Sentinel contract
---

# Sentinel contract

## Contract address

`z1qxemdeddedxsentynelxxxxxxxxxxxxxwy0r2r` — `SentinelAddress`.

## Methods observed

| Method | Inputs | Triggers |
|---|---|---|
| `Revoke` | (none) | Set `sentinels.active = false` for the paired sender. |

New sentinel registrations are picked up by the cached-data sync
(`SentinelApi.GetAllActive`) on its 5-minute cadence, not by a per-block
handler. Only revocations have an explicit event handler.

## Per-method write effects

- **Revoke**
    - `sentinels`: `SetInactiveBatch(owner)` flips `active` to false.

## Special computation

None.

## Tests

No dedicated unit tests. Sentinel registration is exercised via the
cached-data sync path; revocations are integration-level.

## Notes

Sentinel-source rewards land in [`reward_transactions`](../schema/reward_transactions.md)
as `RewardTypeSentinel` (4), routed through `classifyReward` — see
[`rewards.md`](rewards.md).
