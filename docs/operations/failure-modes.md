---
title: Failure modes
---

# Failure modes

Known ways the indexer can stall or misbehave, with detection and
mitigation.

## Subscription stall

**Symptom:** `MAX(momentums.timestamp)` stops advancing while the
container is healthy. Logs show no `received new momentum` for
30+ seconds.

**Cause:** WebSocket subscription dropped silently. The node may have
restarted, or the connection went stale.

**Detection:** [`monitoring.md`](monitoring.md) — the
`seconds_behind` query. Set a Prometheus alert for `> 60`.

**Mitigation:** The SDK auto-reconnects and the indexer re-subscribes
on every connection-established callback. If a subscription is wedged
without the SDK noticing (rare), restart the indexer container.

## Per-momentum batch failure

**Symptom:** Sync stops advancing; logs show
`momentum <h> batch failed: …` followed by a rollback.

**Cause:** A constraint violation in the transactional batch (e.g.,
malformed data from the node, a unique-index conflict that the
ON CONFLICT clause doesn't cover). The whole momentum's writes roll
back and the sync loop retries.

**Detection:** Log filter for `momentum.*batch failed`.

**Mitigation:** The retry loop will pick it up on the next pass. If
the same momentum fails repeatedly, the data is genuinely bad — open
an issue with the failing block hash and the error message.

## Balance updates skipped on big momentums

**Symptom:** `balances` rows for certain addresses are stale. The
`last_updated_timestamp` is older than the most recent block that
touched them.

**Cause:** `processMomentum` skips per-address balance refresh when
`m.tx_count >= genesisBalanceUpdateThreshold` (1000). Genesis and any
hypothetical large block hits this guard.

**Detection:** `SELECT * FROM balances WHERE last_updated_timestamp <
$some_threshold`.

**Mitigation:** This is by design — fetching per-address balances on
genesis would take hours. The flow metrics in
[`accounts`](../schema/accounts.md) are computed from sends/receives
and remain accurate; `balances` is only stale for unusual cases.

## Historical SDK accelerator-types panic

**Symptom:** The cached-data sync hangs on "fetching projects from
accelerator", or the indexer panics with a type-assertion error
referencing accelerator types.

**Cause:** Older znn-sdk-go versions reused go-zenon internal types
for client-side JSON deserialization.

**Detection:** Crash log mentioning `accelerator_types` or a panic in
the cached-data goroutine.

**Mitigation:** The current checkout uses upstream
`github.com/0x3639/znn-sdk-go` directly and should not hit this. If it
returns after an SDK bump, pin or roll back the SDK version and open an
upstream issue. See
[`docs/reference/known-issues.md`](../reference/known-issues.md).

## Bridge sync "unknown network" warnings

**Symptom:** Log lines like `bridge sync: failed to update unwrap
requests {"error": "unknown network"}`.

**Cause:** The bridge API on some test-network configurations returns
this error for certain network classes the indexer hasn't been
configured to ignore.

**Detection:** Recurring WARN log lines on every 1-minute bridge tick.

**Mitigation:** Currently a known issue; the indexer logs the warning
and continues. Wraps and configuration tables stay fresh; only unwraps
miss the refresh.

## Reward indexing produces no rows

**Symptom:** `reward_transactions` table is empty or stops growing
even though the chain is producing rewards.

**Cause:** Two historical variants:

1. Pre-fix BlockType literals were wrong (the indexer used 4 / 6
   instead of `BlockTypeUserReceive` / `BlockTypeContractSend` =
   3 / 4). Fixed in code; legacy data needs the backfill.
2. The reward source contract changed and the classifier doesn't
   recognize it.

**Detection:** `SELECT MAX(momentum_timestamp), reward_type, COUNT(*)
FROM reward_transactions GROUP BY reward_type`.

**Mitigation:** For (1), run
[`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards).
For (2), inspect `classifyReward` in
[`internal/indexer/rewards.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/rewards.go).

## DB connection pool exhaustion

**Symptom:** Sync slows dramatically; logs show `acquire failed:
context deadline exceeded` or similar pgxpool errors.

**Cause:** A goroutine is holding a connection without releasing it.
With the default `database.pool_size = 10`, this manifests under
contention from concurrent batches + the per-block account-info calls.

**Detection:** Slow sync coupled with `pool_size` warnings in logs.

**Mitigation:** Raise `database.pool_size`. The compose default uses
the indexer's default of 10; bump to 20 in `.env` if needed.

## Node returning stale data

**Symptom:** The indexer's `MAX(height)` matches the node's frontier,
but the chain has clearly moved on (observed via a second node).

**Cause:** The node is on a stale fork, behind on its peers, or
intentionally stuck. The indexer trusts every response — it can't
detect this on its own.

**Mitigation:** Switch `NODE_URL_WS` to a known-good node. The
indexer's `MAX(height)` will advance once the new node is ahead.

## Disk full

**Symptom:** Indexer logs ENOSPC or write errors; Postgres logs
"could not extend file".

**Mitigation:**

1. Stop the indexer (`docker compose stop indexer`).
2. Free space — old backups (`./backups/`), old logs.
3. Restart.

The Postgres data volume lives at `./data`. Plan for at least 50 GB
on a production chain.

## Dagger engine out of disk

**Symptom:** Dagger CI runs fail with disk-pressure errors.

**Mitigation:**

```bash
docker stop dagger-engine-* && docker rm dagger-engine-* && docker volume prune -f
```

(Documented in CLAUDE.md historically.)
