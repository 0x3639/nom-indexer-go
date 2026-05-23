---
title: internal/indexer
---

# `internal/indexer`

Source: [`internal/indexer/`](https://github.com/0x3639/nom-indexer-go/tree/main/internal/indexer)

## Package overview

The heart of the service: per-momentum processing, sync + subscription
loop, bridge sync, cron jobs. See [`doc.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/doc.go)
for the package-level docstring.

## Files

| File | Responsibility |
|---|---|
| [`indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go) | `Indexer` type, `Run`, sync + subscription loops, bridge sync, cached-data sync, helpers (`getVotingID`, `getStakeCancelID`, `getFusionCancelID`, `getPillarOwnerAddress`, `getPillarInfoForProducer`, `updateBridgeConfig`). |
| [`processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go) | `processMomentum`, `processAccountBlocks`, `updateBalances`, `safeBigIntToInt64`. The per-momentum transactional pipeline. |
| [`embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go) | `indexEmbeddedContracts` dispatch + per-contract handlers (`indexPillarContract`, `indexStakeContract`, `indexPlasmaContract`, `indexAcceleratorContract`, `indexTokenContract`, `indexSentinelContract`). |
| [`decoder.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder.go) | `tryDecodeTxData`, `tryDecodeFromAbi`, `formatArg`. ABI decoding. |
| [`rewards.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards.go) | `indexLiquidityReward`, `indexReceivedReward`, `classifyReward`. Reward routing. |
| [`cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go) | `runCronLoop`, `runVotingActivity`, `runTokenHolderCounts`, `runStatSnapshots`, `ParseCronInterval`. |
| [`retry.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/retry.go) | `withRetry` — exponential backoff helper for transient RPC/DB errors. |

## Entry points

| Symbol | Used by |
|---|---|
| `NewIndexer(client, pool, logger)` | `cmd/indexer/main.go`. |
| `NewIndexerWithCron(client, pool, logger, CronConfig{…})` | Same — when cron intervals are configured. |
| `Indexer.Run(ctx)` | The main sync + subscription + cron orchestrator. Returns when ctx is cancelled. |
| `Indexer.Backfill(ctx)` | One-shot gap fill. Called by both `BACKFILL_ON_STARTUP=true` and `cmd/backfill`. |
| `Indexer.ProcessMomentumPublic(ctx, m)` | Exported wrapper around `processMomentum` for tooling that needs to process a fetched momentum directly. |
| `Indexer.UpdateCachedDataPublic(ctx)` | Same — for backfill warm-up. |

## See also

- [`docs/architecture/overview.md`](../architecture/overview.md) — goroutine layout.
- [`docs/architecture/data-flow.md`](../architecture/data-flow.md) — per-momentum trace.
- [`docs/indexing/index.md`](../indexing/index.md) — contract dispatch + handler details.
