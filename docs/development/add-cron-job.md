---
title: Add a cron job
---

# Add a cron job

The cron loop in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go)
runs three jobs today: voting activity, token holder counts, and the
1-hour daily stat snapshot. Adding a fourth is short.

## 1. Choose: tunable interval or hardcoded?

| Choice | Why |
|---|---|
| Tunable (new `cron.foo_interval` config field) | The cadence has operational tradeoffs (cost vs. freshness). |
| Hardcoded constant in `cron.go` | The job has one obviously-right interval (e.g., daily snapshots are 1h). |

Both existing tunable jobs (voting / holders) are 10 minutes by
default; the stat snapshot is hardcoded 1 hour. Mirror the closest
analog.

## 2. Add the method

```go
func (i *Indexer) runFoo(ctx context.Context) {
    start := time.Now()
    // ... your logic ...
    // Use repository methods; queue inside a batch if you write
    // multiple rows.
    i.logger.Info("foo refreshed",
        zap.Int("rows", n),
        zap.Duration("duration", time.Since(start)))
}
```

Keep the method on `*Indexer` so it can use `i.repos`, `i.logger`,
`i.pool`, etc.

## 3. Wire it into `runCronLoop`

If tunable, add a config field:

```go
// In CronConfig in indexer.go:
type CronConfig struct {
    VotingActivityInterval time.Duration
    TokenHoldersInterval   time.Duration
    FooInterval            time.Duration   // new
}
```

…and parse it from `internal/config`:

```go
fooInterval, err := indexer.ParseCronInterval(cfg.Cron.FooInterval, 10*time.Minute)
// then pass into NewIndexerWithCron
```

Then in `cron.go`'s `runCronLoop`:

```go
fooTicker := time.NewTicker(fooInterval)
defer fooTicker.Stop()

for {
    select {
    case <-ctx.Done():
        return
    // ... existing cases ...
    case <-fooTicker.C:
        i.runFoo(ctx)
    }
}
```

For hardcoded intervals, define the constant alongside `statsInterval`
in `runCronLoop` and add the ticker the same way.

## 4. Run on startup

The existing pattern in `runCronLoop` calls each job once before the
ticker fires so dashboards have data immediately:

```go
i.runVotingActivity(ctx)
i.runTokenHolderCounts(ctx)
i.runStatSnapshots(ctx)
i.runFoo(ctx)  // new
```

## 5. If the job writes to a daily snapshot table

Use UTC date bucketing — see existing examples in `cron.go`. The
template is:

```go
today := time.Now().UTC().Format("2006-01-02")
todayStart, _ := time.Parse("2006-01-02", today)
startTs := todayStart.Unix()
endTs := startTs + 86400
// Query with WHERE momentum_timestamp >= startTs AND momentum_timestamp < endTs
// Upsert ON CONFLICT (date, …) DO UPDATE SET …
```

The repository helper should match the
[`StatHistoryRepository`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/stat_history.go)
pattern.

## 6. Document it

- Add a row to [`docs/config/cron-intervals.md`](../config/cron-intervals.md).
- Add an entry to
  [`docs/architecture/cron-and-snapshots.md`](../architecture/cron-and-snapshots.md).
- If it writes a new column or table, follow
  [`add-table.md`](add-table.md) instead.

## 7. Test

Pure-logic helpers go in `cron_test.go` next to the existing tests.
Database-touching logic gets an integration test that exercises the
upsert (and idempotent re-run on the same date).
