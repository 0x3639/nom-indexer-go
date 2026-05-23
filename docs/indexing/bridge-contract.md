---
title: Bridge contract
---

# Bridge contract

## Contract address

`z1qxemdeddedxdrydgexxxxxxxxxxxxxxxmqgr0d` — `BridgeAddress`.

## Methods observed

Unlike the other embedded contracts, the bridge has **no per-block ABI
handler** in `embedded.go`. Bridge state lives in three places, all
refreshed by the bridge sync loop (`runBridgeSyncLoop` in `indexer.go`,
1-minute cadence):

| What | API call | Target table(s) |
|---|---|---|
| Pending + finalized wraps | `BridgeApi.GetAllWrapTokenRequests` | [`wrap_token_requests`](../schema/wrap_token_requests.md) |
| Pending + finalized unwraps | `BridgeApi.GetAllUnwrapTokenRequests` | [`unwrap_token_requests`](../schema/unwrap_token_requests.md) |
| Admin, halt state, TSS pubkey | `BridgeApi.GetBridgeInfo` | [`bridge_admin`](../schema/bridge_admin.md) |
| Guardian set + delays | `BridgeApi.GetSecurityInfo` | [`bridge_guardians`](../schema/bridge_guardians.md), [`bridge_security_info`](../schema/bridge_security_info.md) |
| Orchestrator parameters | `BridgeApi.GetOrchestratorInfo` | [`bridge_orchestrator_info`](../schema/bridge_orchestrator_info.md) |
| Networks + token pairs | `BridgeApi.GetAllNetworks` (paginated) | [`bridge_networks`](../schema/bridge_networks.md), [`bridge_network_tokens`](../schema/bridge_network_tokens.md) |

## Per-call write effects

- **Wrap sync** — newest-first paging until we reach the oldest
  unfinalized row already in DB (`GetWrapSyncStopHeight`). Upsert each
  request; on conflict, only `signature` and
  `confirmations_to_finality` mutate.
- **Unwrap sync** — newest-first paging until the oldest unfinalized
  unwrap (where `NOT redeemed AND NOT revoked`); unwraps can finalize
  out of order because users initiate them, so older rows can still need
  signature/redeem/revoke refresh on every pass.
- **Admin / orchestrator / security** — single upsert per tick to the
  singleton tables.
- **Guardians** — upsert each nominated + accepted guardian with
  `last_updated_timestamp = now`, then call
  `MarkGuardiansAbsent(now)` which flips every row older than `now` to
  inactive — the "removed guardian" pattern.
- **Networks** — paginate `GetAllNetworks`; upsert each `BridgeNetwork`,
  and for each network upsert its nested `TokenPairs`.

## Special computation

`MinAmount` on network tokens goes through `safeBigIntToInt64` — int64
cap applies.

## Tests

- [`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go) — `TestIntegration_BridgeConfig_AdminAndGuardiansAndNetworks` covers the singleton, guardian absent-marking, and network upsert paths.
- [`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go) — `TestIntegration_Bridge_FinalityHelpers` covers wrap finalization.

## Notes

The test network's unwrap API returns "unknown network" for some
configurations; the sync logs a warning and continues. See
[`docs/operations/failure-modes.md`](../operations/failure-modes.md).
