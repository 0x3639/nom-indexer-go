---
title: Pillar contract
---

# Pillar contract

## Contract address

`z1qxemdeddedxpyllarxxxxxxxxxxxxxxxsy3fmg` — defined as
`PillarAddress` in
[`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go).

## Methods observed

Handler dispatch lives in `indexPillarContract` in
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go).

| Method | Inputs (decoded) | Triggers |
|---|---|---|
| `Register`, `RegisterLegacy` | `name`, `producerAddress`, `rewardAddress` | Insert into `pillar_updates`; if a descendant burn is present, record `slot_cost_qsr` + `spawn_timestamp` on the new `pillars` row. |
| `UpdatePillar` | `name`, `producerAddress`, `rewardAddress`, plus `pillarOwner` (enriched by the caller) | Insert into `pillar_updates`. |
| `Delegate` | `name` (the pillar's name) | Update `accounts.delegate` + `delegation_start_timestamp`; close any open row in `delegations`; open a new row. |
| `Undelegate` | (none) | Clear `accounts.delegate`; close the open `delegations` row. |
| `Revoke` | `name` | Set `pillars.is_revoked = true` and `revoke_timestamp`, preserving the rest of the row's fields. |

## Per-method write effects

- **Register / RegisterLegacy**
    - `pillar_updates`: append a row with the owner from `block.PairedAccountBlock.Address`.
    - `pillars`: `UpdateSpawnInfoBatch` (only if the descendant burn to the Token contract is detected) sets `spawn_timestamp` and `slot_cost_qsr`. The actual pillar row appears on the next `updateCachedData` tick.
- **UpdatePillar**
    - `pillar_updates`: append a row using the `pillarOwner` input enriched by `processAccountBlocks` (looked up via the cached `pillarNameToOwner` map for the supplied `name`).
- **Delegate**
    - Looks up the target pillar's owner via `getPillarOwnerAddress(name)`.
    - `accounts.delegate` / `delegation_start_timestamp`: updated for the delegator (`block.PairedAccountBlock.Address`).
    - `delegations`: `CloseActiveBatch(delegator, ts)` + `OpenBatch(delegator, pillar_owner, ts)`.
- **Undelegate**
    - `accounts.delegate`: cleared to `''`, `delegation_start_timestamp` reset to 0.
    - `delegations`: `CloseActiveBatch(delegator, ts)` only.
- **Revoke**
    - `pillars`: `SetAsRevokedBatch(owner, name, ts)` — see
      [`schema/pillars.md`](../schema/pillars.md) for the preserved-fields
      contract.

## Special computation

- **Spawn QSR cost.** The Register call doesn't carry the cost directly;
  the handler scans `block.DescendantBlocks` for a Burn descendant to
  the Token contract and reads its `Amount` field.

## Tests

- [`internal/indexer/voting_id_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/voting_id_test.go) — voting_id derivation (also used by accelerator votes).
- Integration tests in
  [`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go)
  cover the `delegations` open/close round-trip and the pillar revoke
  field-preservation invariant.
