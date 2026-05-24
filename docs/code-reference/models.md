---
title: internal/models
---

# `internal/models`

Source: [`internal/models/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/models)

## Package overview

Go struct mirrors of every table in the schema, plus enum types and
address constants. The leaf of the import graph — stdlib only — which
is intentional: `internal/api/dto` and the MCP tool layer can depend on
it without pulling in pgx, viper, zap, or the SDK.

See the [`doc.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/doc.go)
package docstring.

## Files

| File | Contents |
|---|---|
| [`models.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/models.go) | All 30 schema-mirror structs + `RewardType` enum + `NewTxData`. |
| [`constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go) | Embedded contract addresses, special addresses, ZTS standards, chain constants (`MomentumBlockTimeSec`, `FusionExpirationTime`, …). Also `IsEmbeddedContract` and `EmbeddedContractAddresses()`. |

## Structs

The Go struct names match the table names (CamelCase), e.g.:

- `Momentum` ↔ `momentums`
- `AccountBlock` ↔ `account_blocks`
- `TokenMint` ↔ `token_mints`
- `BridgeNetworkToken` ↔ `bridge_network_tokens`

Each struct has `db:"…"` tags matching the column names exactly. See
[`docs/schema/`](../schema/index.md) for the per-table column tables.

## Enums

```go
type RewardType int

const (
    RewardTypeUnknown   RewardType = 0
    RewardTypeStake     RewardType = 1
    RewardTypeDelegation RewardType = 2
    RewardTypeLiquidity RewardType = 3
    RewardTypeSentinel  RewardType = 4
    RewardTypePillar    RewardType = 5
)
```

The `String()` method is also defined for log-friendly rendering.

## Constants

| Group | Constants |
|---|---|
| Embedded contract addresses | `PlasmaAddress`, `PillarAddress`, `TokenAddress`, `SentinelAddress`, `StakeAddress`, `AcceleratorAddress`, `SwapAddress`, `LiquidityAddress`, `BridgeAddress`, `HtlcAddress`, `SporkAddress` |
| Special addresses | `EmptyAddress`, `LiquidityTreasuryAddress` |
| Token standards | `EmptyTokenStandard`, `ZnnTokenStandard`, `QsrTokenStandard` |
| Chain constants | `GenesisMomentumTime`, `MomentumBlockTimeSec`, `FusionExpirationTime`, `FusionExpirationBlocks` |
| Thresholds | (defined in `internal/indexer/processor.go`: `genesisBalanceUpdateThreshold`) |

See [`docs/reference/addresses.md`](../reference/addresses.md) for the
full table.

## Helpers

```go
func IsEmbeddedContract(addr string) bool
func EmbeddedContractAddresses() []string
```

Both used by `decoder.tryDecodeTxData` to decide whether to attempt
ABI decoding for a given block's `ToAddress`.

## See also

- [`docs/schema/`](../schema/index.md) — the table-by-table contract
  these structs mirror.
- [`docs/reference/addresses.md`](../reference/addresses.md) — every
  constant with context.
