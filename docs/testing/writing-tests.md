---
title: Writing tests
---

# Writing tests

Patterns to follow when adding tests to the codebase.

## Table-driven tests for pure helpers

Example from
[`internal/indexer/retry_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/retry_test.go):

```go
func TestParseCronInterval(t *testing.T) {
    tests := []struct {
        name string
        in   string
        def  time.Duration
        want time.Duration
    }{
        {"empty falls back to default", "", 10 * time.Minute, 10 * time.Minute},
        {"valid duration parsed", "30s", time.Hour, 30 * time.Second},
        {"invalid returns default", "garbage", 5 * time.Minute, 5 * time.Minute},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ParseCronInterval(tt.in, tt.def)
            if got != tt.want {
                t.Errorf("ParseCronInterval(%q, %v) = %v, want %v",
                    tt.in, tt.def, got, tt.want)
            }
        })
    }
}
```

Each case is its own subtest (`t.Run`). Subtests make failure output
easy to grep — `--- FAIL: TestX/invalid_returns_default`.

## Repository round-trip tests (integration)

Example pattern from
[`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go):

```go
//go:build integration

func TestIntegration_Momentum_RoundTrip(t *testing.T) {
    pool := newTestDB(t)
    repo := NewMomentumRepository(pool)
    ctx := t.Context()

    want := &models.Momentum{
        Height: 42, Hash: "abc…", Timestamp: 1700000000,
    }
    require.NoError(t, repo.Insert(ctx, want))

    got, err := repo.GetByHeight(ctx, 42)
    require.NoError(t, err)
    if got.Hash != want.Hash {
        t.Errorf("Hash = %q, want %q", got.Hash, want.Hash)
    }
}
```

Always start with `newTestDB(t)` to TRUNCATE. Use the shared
`sendBatch(t, ctx, pool, batch)` helper for batched inserts.

## Batched-write tests

When testing repository code that queues into a `*pgx.Batch`:

```go
batch := &pgx.Batch{}
repo.InsertBatch(batch, &models.Foo{ /* … */ })
repo.InsertBatch(batch, &models.Foo{ /* … */ })
sendBatch(t, ctx, pool, batch)

// Now assert on the resulting rows.
```

`sendBatch` (defined in
[`integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go))
sends the batch, drains the results, and fails the test on any error.

## Logger fakes

The `withRetry` tests use `zap.NewNop()` to silence output. For tests
that need to assert on log lines, use `zaptest.NewObserver()`:

```go
core, recorded := observer.New(zapcore.WarnLevel)
logger := zap.New(core)
// run code that logs ...
got := recorded.FilterMessage("transient error").Len()
```

See
[`internal/indexer/processor_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor_test.go)
for the `safeBigIntToInt64` test that uses observed logs.

## Testing time-dependent code

Don't use `time.Now()` directly in code under test if you can avoid
it — accept a `time.Time` parameter. For cron jobs that compute
"today", inject a date string parameter rather than calling
`time.Now().UTC().Format(...)` inline.

If the production code's time use can't be cleanly parameterized,
use a test where the wall-clock dependency is acceptable (e.g., bound
to "within the last hour" rather than an exact instant).

## Mocking RPC

We do **not** mock RPC. The `znn-sdk-go` types are large and don't
have natural test doubles. Either:

- The code under test doesn't call RPC (pure helper). Test directly.
- The code calls RPC but the test exercises a different code path
  (e.g., DB-only assertions on a pre-populated table). Set up the DB
  state and skip the RPC entirely.
- The code requires a real RPC call. Don't write the test; verify
  manually against a node.

## Goldens

The decoder tests use real SDK ABIs (`embedded.Pillar`,
`embedded.Accelerator`) rather than hand-crafted byte sequences. This
is the closest thing to golden files in this repo — the test relies
on the SDK staying ABI-stable.

If the SDK changes ABI shape, the decoder tests will fail loudly. Fix
the test by re-encoding through the new SDK and asserting on the new
decoded shape.

## `t.Helper()`

Mark non-test helper functions with `t.Helper()` so failure locations
point at the calling test, not the helper. Convention used throughout
`newTestDB`, `sendBatch`, etc.

## Coverage

`go test -cover` is the canonical measure. Don't optimize for coverage
metrics — write tests that catch real regressions.
