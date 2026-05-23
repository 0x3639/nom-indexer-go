---
title: Addresses and token standards
---

# Addresses and token standards

Every constant in
[`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go).

## Embedded contract addresses

| Constant | Address | Purpose |
|---|---|---|
| `PlasmaAddress` | `z1qxemdeddedxplasmaxxxxxxxxxxxxxxxxsctrp` | Plasma fusion contract (`Fuse`, `CancelFuse`). |
| `PillarAddress` | `z1qxemdeddedxpyllarxxxxxxxxxxxxxxxsy3fmg` | Pillar registry + delegation + revocation. Source of pillar + delegate rewards. |
| `TokenAddress` | `z1qxemdeddedxt0kenxxxxxxxxxxxxxxxxh9amk0` | Token issue / mint / burn. |
| `SentinelAddress` | `z1qxemdeddedxsentynelxxxxxxxxxxxxxwy0r2r` | Sentinel registration. Source of sentinel rewards. |
| `StakeAddress` | `z1qxemdeddedxstakexxxxxxxxxxxxxxxxjv8v62` | Staking. Source of stake rewards. |
| `AcceleratorAddress` | `z1qxemdeddedxaccelerat0rxxxxxxxxxxp4tk22` | Accelerator-Z (projects, phases, votes). |
| `SwapAddress` | `z1qxemdeddedxswapxxxxxxxxxxxxxxxxxxl4yww` | Cross-token swap (not indexed today — no per-method handler). |
| `LiquidityAddress` | `z1qxemdeddedxlyquydytyxxxxxxxxxxxxflaaae` | Liquidity program. Source of liquidity rewards. |
| `BridgeAddress` | `z1qxemdeddedxdrydgexxxxxxxxxxxxxxxmqgr0d` | Bridge wrap/unwrap. |
| `HtlcAddress` | `z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw` | Hash time-locked contracts (not indexed today). |
| `SporkAddress` | `z1qxemdeddedxsp0rkxxxxxxxxxxxxxxxx956u48` | Spork governance (not indexed today). |

## Special addresses

| Constant | Address | Meaning |
|---|---|---|
| `EmptyAddress` | `z1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsggv2f` | Sentinel "no destination" address used in reward-receive detection. |
| `LiquidityTreasuryAddress` | `z1qqw8f3qxx9zg92xgckqdpfws3dw07d26afsj74` | The on-chain treasury that pays out liquidity rewards. Detected by the `indexLiquidityReward` branch in `processAccountBlocks`. |

## Token standards

| Constant | ZTS | Meaning |
|---|---|---|
| `EmptyTokenStandard` | `zts1qqqqqqqqqqqqqqqqtq587y` | Sentinel "no token transfer". |
| `ZnnTokenStandard` | `zts1znnxxxxxxxxxxxxx9z4ulx` | ZNN — Zenon's primary token. |
| `QsrTokenStandard` | `zts1qsrxxxxxxxxxxxxxmrhjll` | QSR — Zenon's secondary token (plasma + secondary rewards). |

## Chain constants

| Constant | Value | Meaning |
|---|---|---|
| `GenesisMomentumTime` | `1637755210` (Nov 24, 2021) | Unix timestamp of genesis. |
| `MomentumBlockTimeSec` | `10` | Target seconds-per-block. |
| `FusionExpirationTime` | `3600` (1 hour) | Wall-clock window before a fusion becomes cancellable. |
| `FusionExpirationBlocks` | `360` | Same window expressed as blocks (`3600 / 10`). |

## Block types

From the SDK
([`github.com/0x3639/znn-sdk-go/utils`](https://github.com/0x3639/znn-sdk-go)),
not from this repo's constants:

| Constant | Value | Meaning |
|---|---|---|
| `BlockTypeGenesisReceive` | 1 | Genesis-block receives. |
| `BlockTypeUserSend` | 2 | User-initiated send. |
| `BlockTypeUserReceive` | 3 | User-initiated receive. |
| `BlockTypeContractSend` | 4 | Contract-initiated send (e.g., reward mint). |
| `BlockTypeContractReceive` | 5 | Contract-initiated receive. |

These were the source of a historical indexing bug — see
[`known-issues.md`](known-issues.md).

## Helpers

| Function | Returns |
|---|---|
| `models.EmbeddedContractAddresses()` | All 11 embedded contract addresses as a slice. |
| `models.IsEmbeddedContract(addr)` | `true` if `addr` matches any embedded contract. Used by the decoder to decide whether to attempt ABI decoding. |
