---
title: Plasma contract
---

# Plasma contract

## Contract address

`z1qxemdeddedxplasmaxxxxxxxxxxxxxxxxsctrp` — `PlasmaAddress`.

## Methods observed

Handler: `indexPlasmaContract` in `embedded.go`.

| Method | Inputs | Triggers |
|---|---|---|
| `Fuse` | `address` (beneficiary; optional) | Insert into [`fusions`](../schema/fusions.md). |
| `CancelFuse` | `id` | Mark the fusion inactive. |

## Per-method write effects

- **Fuse**
    - `id` is the paired send block's hash.
    - `beneficiary` is the `address` input if present, else the paired
      send block's sender.
    - `qsr_amount` is the paired send block's `Amount`.
    - `expiration_height = momentum_height + FusionExpirationBlocks`
      (constant from
      [`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go),
      approximately 360 blocks for a 1-hour window).
    - `cancel_id` from `getFusionCancelID(id)` — ABI roundtrip on the
      Plasma contract's `CancelFuse(id)` signature.
- **CancelFuse**
    - Input `id` is the original fusion ID.
    - `cancel_id = getFusionCancelID(id)`; `SetInactiveBatch(cancel_id,
      address)`.

## Special computation

`cancel_id` mirrors the stake-contract pattern but uses
`embedded.Plasma.EncodeFunction("CancelFuse", …)` instead of the Stake
ABI.

`expiration_height` is **approximate**, computed from a fixed
seconds-per-block constant. The contract's actual expiry enforcement is
authoritative; this column is a UI hint.

## Tests

- [`internal/indexer/voting_id_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/voting_id_test.go)
  covers `getFusionCancelID` indirectly.
