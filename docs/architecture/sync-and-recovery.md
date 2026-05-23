---
title: Sync and recovery
---

# Sync and recovery

How the indexer catches up from a cold start, transitions to real-time
mode, and recovers from connection drops.

## Two modes

| Mode | When | Driver |
|---|---|---|
| **Catch-up sync** | `MAX(momentums.height) < frontier height` | `Indexer.sync(ctx)` |
| **Subscription** | caught up to frontier | `Indexer.runSubscriptionLoop(ctx)` |

`Run` calls `sync` once, then enters the subscription loop. The
subscription loop falls back to a catch-up `sync` on every
disconnection.

## Catch-up sync

```go
func (i *Indexer) sync(ctx context.Context) error {
    for {
        dbHeight := i.repos.Momentum.GetLatestHeight(ctx)   // wrapped in withRetry
        frontier := i.client.LedgerApi.GetFrontierMomentum() // wrapped in withRetry
        if dbHeight >= frontier.Height { return nil }       // caught up

        startHeight := dbHeight + 1
        momentums := i.client.LedgerApi.GetMomentumsByHeight(startHeight, 100)
        for _, m := range momentums.List {
            if err := i.processMomentum(ctx, m); err != nil { return err }
        }
    }
}
```

Key properties:

- **Batch of 100.** The Ledger API supports `count` up to ~100 per
  call; the indexer uses the max.
- **Sequential momentum processing.** No parallelism inside the
  batch — Postgres write order is monotonic-by-height, which makes
  `MAX(height)` a reliable cursor.
- **Cached-data refresh.** Every 1000 heights, the loop calls
  `updateCachedData` to keep the pillar / sentinel / project cache
  hot during long catch-up runs.
- **Genesis edge case.** If `dbHeight == 0`, `startHeight = 1`. Genesis
  has a special block-type (`BlockTypeGenesisReceive = 1`) handled by
  the same `processMomentum` code path.

## Subscription mode

```go
func (i *Indexer) runSubscriptionLoop(ctx context.Context) error {
    for {
        err := i.runSubscriptionSession(ctx)
        if ctx.Err() != nil { return ctx.Err() }
        i.logger.Warn("subscription ended; catch-up + resubscribe", zap.Error(err))
        i.sync(ctx)              // catch up before resubscribing
    }
}
```

Each session:

1. `client.SubscriberApi.ToMomentums(ctx)` returns a channel.
2. The loop reads momentum events off the channel, fetches each via
   `GetMomentumsByHeight(h, 1)`, and processes.
3. Returns either when ctx is cancelled, when the channel closes, or
   when a "restart subscription" signal arrives via `restartSubCh`.

The `restartSubCh` is fed by the SDK's `OnConnectionEstablished`
callback — after a reconnect, the current subscription is dead and
needs to be re-created on the fresh underlying connection.

## Reconnect handling

The SDK owns the WebSocket lifecycle. Two callbacks are wired in
`Indexer.Run`:

- `AddOnConnectionEstablishedCallback` — signals `restartSubCh` so the
  subscription session ends and a new one starts on the fresh
  connection.
- `AddOnConnectionLostCallback` — logs a warning. The SDK reconnects
  automatically.

Between callbacks, the indexer keeps trying to read from its current
subscription channel. The catch-up sync that runs before
re-subscription ensures no momentums are missed during the disconnect
window.

```mermaid
sequenceDiagram
    participant SDK as SDK
    participant Sub as runSubscriptionSession
    participant Loop as runSubscriptionLoop
    participant Sync as sync()

    Note over SDK,Sub: connection alive
    SDK->>Sub: momentum events
    Sub->>Sub: processMomentum

    Note over SDK: connection lost
    SDK->>SDK: reconnect
    SDK->>Sub: subscription channel closes / restart signal
    Sub-->>Loop: return (err or nil)
    Loop->>Sync: catch-up
    Sync-->>Loop: done
    Loop->>Sub: new session
```

## Retry helper

`withRetry` in
[`internal/indexer/retry.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/retry.go)
wraps RPC + DB calls in the catch-up loop:

- 6 attempts.
- Exponential backoff starting at 500 ms, capped at 30 s.
- Cancellable via ctx.
- Returns `fmt.Errorf("%s failed after %d attempts: %w", label,
  attempts, lastErr)` on giveup.

Wrapped sites: `GetLatestHeight`, `GetFrontierMomentum`,
`GetMomentumsByHeight`. Per-block calls inside `processMomentum`
(`GetAccountBlockByHash`, `GetAccountInfoByAddress`) are not wrapped
— they're fast enough that a per-call retry adds latency without
helping much; the per-momentum rollback handles their failure modes.

## Backfill

Two parallel paths for filling gaps in `momentums`:

- `BACKFILL_ON_STARTUP=true` runs `Indexer.Backfill` at startup, before
  entering the main sync loop.
- `cmd/backfill` runs the same code path standalone, against a
  running indexer.

The backfill query identifies both "missing height" gaps and
"incomplete momentum" rows (`tx_count > 0 AND actual_account_blocks <
tx_count`). See [`docs/operations/backfill.md`](../operations/backfill.md).

## What can go wrong

- **Node returns stale data.** The indexer follows the node. If the
  node is on a stuck fork, the indexer's `MAX(height)` reflects that.
  Detection: compare against a second node.
- **Per-momentum batch failure.** Transaction rolls back, sync retries
  the height. If the same height keeps failing, the data is genuinely
  bad — open an issue.
- **DB connection pool exhaustion.** Goroutines acquire connections;
  if `pool_size` is too small under load, ops time out. See
  [`docs/operations/failure-modes.md`](../operations/failure-modes.md).
