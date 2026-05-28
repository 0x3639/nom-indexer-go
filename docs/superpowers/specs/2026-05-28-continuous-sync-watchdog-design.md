---
title: Continuous sync watchdog with node failover
date: 2026-05-28
status: design
---

# Continuous sync watchdog with node failover

## Problem

The indexer occasionally drifts out of sync with its znnd source node, and
znnd itself occasionally drifts behind the chain. Both failures are usually
triggered by transient internet loss. Today's only recovery path is a full
stack restart, which is heavy-handed for a problem the process can detect
and correct itself.

The existing mechanism handles one slice of this well: when the SDK detects
a websocket disconnect, it fires `AddOnConnectionEstablishedCallback`, which
calls `signalSubscriptionRestart()` in `internal/indexer/indexer.go`. The
subscription loop exits, re-runs the batched `sync()` catch-up, and
resubscribes. That covers clean disconnects.

What's not covered:

- **Silent stalls.** The websocket stays "alive" from the SDK's perspective
  but stops delivering momentums. Nothing notices until something else
  trips.
- **Local znnd lag.** If znnd itself falls behind the chain (its peers
  drop, its own sync stalls), the indexer keeps consuming the (sparse)
  momentums it receives without any signal that the upstream is degraded.
- **No alternate node.** When local znnd is unrecoverable in the short
  term, the indexer has nowhere to go.

## Goals

1. Detect indexer-vs-znnd drift independent of websocket events, and
   recover by reusing the existing `signalSubscriptionRestart()` path.
2. Detect znnd-vs-chain drift via `stats.syncInfo`, surface it as a
   metric, in the API's existing `/readyz`, and on a new indexer-side
   `/readyz`.
3. Fail over to a configured alternate node when the active node is
   unrecoverable, then fail back to higher-priority nodes once they
   recover (hysteresis to prevent flapping).
4. Make all of the above observable via a single-row
   `indexer_sync_status` table that any consumer can read.

## Non-goals

- Active recovery of znnd itself (e.g., restarting the znnd container
  from inside the indexer). Out of scope; high blast radius.
- Multi-chain support. We verify chain identity (genesis hash) on switch to *prevent*
  cross-chain writes, not to support them.
- Sub-second failover. Defaults target ~60s drift detection and
  ~2.5min failback (asymmetric).
- Backfilling historical gaps. The existing `cmd/backfill` tool owns
  that; this design only handles forward sync.

## Architecture

One new goroutine inside `cmd/indexer`, modeled on the existing
`runBridgeSyncLoop` / `runCachedDataSyncLoop` patterns. It owns both
drift detection and node selection. The SDK client becomes an
`atomic.Pointer` so all goroutines see the current pick without a lock.

```
                         ┌──────────────────────────────────────────────────────┐
                         │                cmd/indexer (process)                 │
                         │                                                      │
   node[0] (local znnd) ─┤   ┌────────────────────────────┐                    │
   ws://znnd:35998       │   │ activeClient                │  ← atomic.Pointer  │
                         │   │  *rpc_client.RpcClient      │     (used by all)  │
   node[1] (remote) ─────┤   └────────────┬───────────────┘                    │
   wss://my.hc1node:35998│                │                                     │
                         │                ▼                                     │
                         │     subscription / bridge / cron / cached            │
                         │     (read activeClient, then RPC)                    │
                         │                                                      │
                         │   ┌───────────────────────────────────────────┐     │
                         │   │ sync watchdog goroutine (30s tick)         │     │
                         │   │   1. probe active: syncInfo + frontier     │     │
                         │   │   2. classify state                        │     │
                         │   │   3. on indexer_lagging:                   │     │
                         │   │        signalSubscriptionRestart()         │     │
                         │   │   4. on node_lagging > N ticks:            │     │
                         │   │        probe higher-priority candidates    │     │
                         │   │        verify chain identity (genesis hash), atomic swap        │     │
                         │   │   5. on fallback AND primary healthy       │     │
                         │   │      for M consecutive ticks: fail back    │     │
                         │   │   6. upsert indexer_sync_status row        │     │
                         │   │   7. update in-mem state for /readyz       │     │
                         │   └───────────────────────────────────────────┘     │
                         │                                                      │
                         │   HTTP listener (NEW)  /healthz, /readyz             │
                         └──────────────────────────────────────────────────────┘

   cmd/api ── reads indexer_sync_status row ──► existing /healthz returns 503
                                                when state ≠ synced for > N ticks
```

## Components

| # | Path | Type | Purpose |
|---|---|---|---|
| 1 | `internal/indexer/watchdog.go` | new | `runSyncWatchdogLoop(ctx, cfg)`. Owns the tick, classification, and reaction. |
| 2 | `internal/indexer/syncinfo.go` | new | Raw JSON-RPC wrapper around the SDK client for `stats.syncInfo`. Returns `{state, currentHeight, targetHeight}` (verified against live znnd: `{"state":2,"currentHeight":N,"targetHeight":N}`; `state=2` means synced). |
| 3 | `internal/indexer/nodepool.go` | new | Holds ordered `[]NodeConfig{URL, Label}`. `Probe(ctx, idx)` returns `{frontier, target, genesisHash, latency, err}`. The `genesisHash` is fetched via `ledger.getMomentumsByHeight(1, 1)` and used as the chain identifier (since `stats.syncInfo` does not expose one). Caches probe clients per URL. |
| 4 | `internal/indexer/client_switch.go` | new | `swapClient(ctx, idx)`: builds new client, verifies chain identity (genesis hash) matches stored, atomically stores, signals restart, closes old client after grace period. |
| 5 | `internal/health/server.go` | new | Tiny `net/http` listener: `/healthz` (process alive) + `/readyz` (in-mem watchdog state). |
| 6 | `internal/models/sync_status.go` | new | `SyncStatus` struct. |
| 7 | `internal/repository/sync_status.go` | new | `Upsert(ctx, s)`, `Get(ctx)`. Single-row table. |
| 8 | `migrations/0NNN_indexer_sync_status.up.sql` | new | Schema (below). |
| 9 | `internal/indexer/indexer.go` | edit | Replace `client *RpcClient` field with `activeClient atomic.Pointer[*RpcClient]`. Update all read sites. Start watchdog + health server in `Run()`. |
| 10 | `internal/config/config.go` | edit | Add `Nodes []NodeConfig` and `Watchdog WatchdogConfig`. Keep `NODE_URL_WS` env back-compat (becomes `nodes[0].url`). Add `NODE_URL_FALLBACKS` env (comma-separated). |
| 11 | `internal/api/router/router.go` | edit | Existing `readyz` handler (already does DB+schema-version checks) joins `indexer_sync_status` row; returns 503 when `state != synced` AND `consecutive_bad_checks >= unhealthy_streak`. Body includes active node label. Bumps `minSchemaVersion` constant from `12` to the new migration number. |
| 12 | `docker-compose.yml` | edit | Indexer healthcheck pointed at new port. New env vars wired. Internal-only port (no host mapping). |
| 13 | `config.yaml.example` | edit | Document the new keys. |

Net: ~5 new Go files, ~5 edited files, 1 migration, 1 example config update. ~600 LOC including tests.

## Data flow

### Watchdog tick (steady state)

Every `interval` (default 30s):

```
1. activeIdx ← syncState.activeNodeIndex
2. probe ← nodePool.Probe(ctx, activeIdx)
     - ledger.getFrontierMomentum    (existing SDK call)
     - stats.syncInfo                (raw JSON-RPC via SDK client)
     - chain identifier              (whichever field syncInfo returns; verify on switch)
3. dbHeight ← momentumRepo.GetLatestHeight(ctx)
4. classify(probe, dbHeight) → exactly one, evaluated in this priority
   (first match wins):
     1. probe_failed         (probe.err != nil)
     2. stalled              (now - syncState.lastProgressAt > stall_threshold)
     3. node_lagging         (probe.target - probe.frontier > node_drift_threshold)
     4. indexer_lagging      (probe.frontier - dbHeight > indexer_drift_threshold)
     5. synced               (none of the above)

   Rationale: probe_failed wins because we can't trust the other fields.
   stalled wins over node_lagging because a stall is actionable
   immediately (restart subscription), whereas node_lagging is something
   we accumulate a streak on. node_lagging wins over indexer_lagging
   because if znnd itself is behind, the indexer-side "drift" is just
   us correctly waiting for znnd.
5. record(probe, classification) → upsert indexer_sync_status row
6. update in-mem syncState (used by /readyz)
7. react(classification, activeIdx) → see table below
```

`lastProgressAt` is updated by the **subscription loop** itself (every
time `processMomentum` commits). The watchdog only reads it. This keeps
the stall detector cheap and independent of probe RPCs.

`lastProgressAt` is **initialized to the indexer start time** (set in
`Indexer.Run()` before the watchdog launches). Otherwise the first
watchdog tick after a fresh process start would see a zero value and
immediately classify as `stalled`. The first real momentum commit
overwrites the start time on its way through.

### Reaction table

| Classification | Side effect |
|---|---|
| **synced** | `streaks[activeIdx].unhealthy = 0`; `streaks[activeIdx].healthy++`. If `activeIdx > 0`, probe higher-priority candidates and update *their* `streaks[idx].healthy` — if any reaches `failback_streak`, attempt failback. |
| **indexer_lagging** | Call `signalSubscriptionRestart()`. Same node, same client — existing reconnect path catches us up. Do not touch `streaks[activeIdx]`; the indexer is the laggard, not the node. |
| **node_lagging** | `streaks[activeIdx].unhealthy++`; `streaks[activeIdx].healthy = 0`. If `streaks[activeIdx].unhealthy ≥ unhealthy_streak`, attempt failover. Do not restart subscription — data we have is fine. |
| **stalled** | First tick in this state: `signalSubscriptionRestart()` (cheap; might fix). Also `streaks[activeIdx].unhealthy++`; if it reaches `unhealthy_streak`, failover. |
| **probe_failed** | `streaks[activeIdx].unhealthy++`. After `unhealthy_streak` consecutive failures, failover. |

**Streak storage.** `syncState.streaks` is a `map[int]NodeStreaks`
keyed by node index (`{healthy, unhealthy int}`). Only the watchdog
goroutine writes to it. A successful failover or failback resets
**all** entries to zero — fresh slate on the new active node.

### Failover

```
1. For idx in (active+1 .. len(nodes)-1):
     probe ← nodePool.Probe(ctx, idx)
     if probe.err || (probe.target - probe.frontier) > node_drift_threshold:
         continue
     if genesisHash_stored != probe.genesisHash:
         log.Error("chain identity (genesis hash) mismatch", refuse switch); continue
     break
2. If no candidate found:
     log.Error("no healthy fallback"); stay on active; keep degrading /readyz.
3. swapClient(idx):
     a. new ← rpc_client.NewRpcClient(nodes[idx].URL)
     b. sanity: call stats.syncInfo on `new`
     c. activeClient.Store(new)                              # atomic publish
     d. signalSubscriptionRestart()                          # tear down WS on old
     e. syncState.activeNodeIndex = idx; failedOverAt = now
     f. upsert sync_status row (active_node_url/label)
     g. close oldClient after 60s grace (longer than `withRetry`'s ~32s
        worst-case retry sequence, so no in-flight RPC sees a closed
        client mid-call)
4. Subscription loop's catch-up sync runs against the new client.
```

### Failback (from the `synced` branch when on fallback)

```
1. For idx in (0 .. active-1):                              # higher priority only
     probe ← nodePool.Probe(ctx, idx)
     if probe.healthy AND probe.genesisHash == stored:
         healthyStreak[idx]++
         if healthyStreak[idx] >= failback_streak:
             swapClient(idx); reset all healthyStreaks; break
     else:
         healthyStreak[idx] = 0
```

Asymmetric thresholds: `unhealthy_streak=2` (~60s to failover),
`failback_streak=5` (~2.5min sustained primary health to failback).
Prevents one good probe from yanking us back during a flaky window.

### Startup

```
1. Indexer.Run() constructs nodePool from config.nodes.
2. activeClient ← nodes[0] client.
3. Synchronous initial watchdog probe BEFORE existing sync():
     a. If primary probes healthy + chain identity (genesis hash) matches stored → proceed.
     b. If primary fails + a fallback succeeds → swap, then sync().
     c. If chain identity (genesis hash) not yet stored (fresh DB) → record primary's
        chain identity (genesis hash) as the canonical value.
     d. If all candidates fail → fatal error, exit (existing config-error pattern).
4. Existing sync() runs against the (possibly already failed-over) active client.
5. Subscription loop starts.
6. Watchdog goroutine starts (independent 30s ticker).
```

### What deliberately does *not* happen on failover

- We do not drop or re-fetch already-indexed momentums. The DB is the
  source of truth; the new node provides from `db_height + 1` forward.
- We do not tear down or reopen the Postgres pool.
- The bridge / cron / cached-data loops get no special treatment — they
  `Load()` the active client on their next tick and proceed.
- We never run a partial switch (e.g., subscription on one node, RPCs on
  another). One active client at a time keeps reasoning simple.

## Concurrency contract

- `activeClient` is `atomic.Pointer[rpc_client.RpcClient]`. Every RPC
  site reads `c := activeClient.Load()` then `c.LedgerApi…`.
- A swap is **client publish + restart signal**. Goroutines mid-RPC
  finish against the old client (safe — old client is closed only after
  a 60s grace, longer than `withRetry`'s ~32s worst case). Their *next*
  RPC sees the new client.
- The subscription is the only stateful consumer; the restart signal is
  what flushes it.
- The watchdog and the SDK's connection-established callback both call
  `signalSubscriptionRestart()`. Already safe — size-1 buffered channel,
  non-blocking send, coalescing.
- `syncState` is mutated only by the watchdog goroutine. `/readyz`
  reads via `sync.RWMutex`-protected snapshot copy.

## Schema

```sql
-- migrations/0NNN_indexer_sync_status.up.sql
CREATE TABLE indexer_sync_status (
    id                     SMALLINT PRIMARY KEY CHECK (id = 1),
    db_height              BIGINT       NOT NULL,
    znnd_frontier_height   BIGINT       NOT NULL,
    znnd_target_height     BIGINT       NOT NULL,
    drift_momentums        BIGINT       NOT NULL,        -- znnd_frontier - db_height
    node_lag_momentums     BIGINT       NOT NULL,        -- znnd_target - znnd_frontier
    state                  TEXT         NOT NULL,        -- synced | indexer_lagging | node_lagging | stalled | probe_failed
    consecutive_bad_checks INTEGER      NOT NULL DEFAULT 0,
    active_node_url        TEXT         NOT NULL,
    active_node_label      TEXT         NOT NULL,
    chain_identifier       TEXT         NOT NULL,         -- genesis momentum hash, set on first probe
    failed_over_at         BIGINT,                        -- unix seconds; null if on primary
    last_progress_at       BIGINT       NOT NULL,         -- unix seconds; last processMomentum commit
    checked_at             BIGINT       NOT NULL          -- unix seconds; this tick
);

-- Downgrade
-- DROP TABLE indexer_sync_status;
```

All times are Unix seconds (BIGINT) to match the project-wide convention
documented in `docs/schema/conventions.md`. Heights are BIGINT (int64) —
overflow is impossible at any realistic chain height.

## Config

```yaml
indexer:
  nodes:
    - url: ws://znnd:35998              # primary (local)
      label: local-znnd
    - url: wss://my.hc1node.com:35998   # fallback (WS)
      label: hc1node-remote-ws
    - url: https://my.hc1node.com:35997 # second-tier fallback (HTTP — probe-only when active)
      label: hc1node-remote-http
  watchdog:
    enabled: true
    interval: 30s
    stall_threshold: 60s            # silent stall before flagging
    indexer_drift_threshold: 3      # momentums DB-behind-znnd before flagging
    node_drift_threshold: 3         # momentums target-minus-current before flagging
    unhealthy_streak: 2             # ticks bad before failover (~60s)
    failback_streak: 5              # ticks good on higher-priority before failback (~2.5min)
  health:
    enabled: true
    port: 9092                      # /healthz, /readyz (internal only)
```

Env-var equivalents (back-compat with current `NODE_URL_WS`):

| Env | Maps to |
|---|---|
| `NODE_URL_WS` | `nodes[0].url` (primary, required) |
| `NODE_URL_FALLBACKS` | comma-separated, populates `nodes[1..]` |
| `INDEXER_HEALTH_PORT` | `health.port` |
| `INDEXER_WATCHDOG_INTERVAL` | `watchdog.interval` (Go duration string) |

If `nodes:` is set in `config.yaml`, it wins over the env vars (existing
precedence rule from `docs/config/reference.md`).

## API `/readyz` extension

The API already separates `/healthz` (process alive, no checks — stays as
is) and `/readyz` (substantive: DB ping + schema-version check via
`minSchemaVersion`). We extend `/readyz`.

New behavior:

```go
// Pseudocode added to existing readyz handler in internal/api/router/router.go
// after the existing DB-version check passes:

sync, err := repo.SyncStatus.Get(ctx)
if errors.Is(err, sql.ErrNoRows) {
    // First run: no watchdog row yet. Don't fail readiness.
    return 200, {"status": "ok"}
}
if err != nil {
    return 503, {"status": "degraded", "reason": "sync_status_unreadable"}
}
if sync.State != "synced" && sync.ConsecutiveBadChecks >= cfg.UnhealthyStreak {
    return 503, {
        "status": "degraded",
        "state":  sync.State,
        "drift":  sync.DriftMomentums,
        "node":   sync.ActiveNodeLabel,
    }
}
return 200, {
    "status": "ok",
    "node":   sync.ActiveNodeLabel,
    "drift":  sync.DriftMomentums,
}
```

The new migration touches `indexer_sync_status` (a new table the API
reads in `/readyz`), so `minSchemaVersion` in `internal/api/router/router.go`
must be bumped from `12` to the new migration number in the same change.

The MCP server's `/readyz` gets the same treatment for parity.

## Indexer `/readyz` semantics

| State | Status code | Body |
|---|---|---|
| `synced` (any node) | 200 | `{"status":"ready","node":"<label>"}` |
| `synced` on fallback | 200 | same (operators see degraded via `active_node_label`) |
| `indexer_lagging` | 503 | `{"status":"draining","reason":"indexer_lagging","drift":N}` |
| `node_lagging` (still on it) | 503 | `{"status":"draining","reason":"node_lagging","node":"<label>"}` |
| `stalled` | 503 | `{"status":"draining","reason":"stalled"}` |
| `probe_failed` (transient) | 200 until `unhealthy_streak`, then 503 | — |

`/healthz` always returns 200 if the process is alive (used to distinguish
"process dead" from "process struggling"). Container restart policy uses
`/healthz`; external monitoring / dashboards use `/readyz`.

## Error handling

- **`stats.syncInfo` not supported by the node.** Some node builds may
  not expose it. On first probe failure for that specific RPC, log
  WARN and fall back to a frontier-only classification (treat node as
  `synced` if probe.frontier matches expectation). Configurable via
  `watchdog.tolerate_missing_syncinfo: true` (default true) — refuses
  to start if false and RPC unavailable.
- **Probe transient errors.** `withRetry` (existing) wraps each probe
  RPC: 3 attempts, 0.5s → 2s backoff. Watchdog cadence (30s) absorbs
  the latency.
- **DB upsert failure.** Log WARN, continue. The in-mem state is still
  authoritative for `/readyz`; only external observers lose this tick.
- **Chain-id mismatch on swap.** Hard refuse the switch, log ERROR,
  keep using the previous client, count it as a probe failure for the
  candidate. Operator must intervene (the candidate is misconfigured).
- **All candidates fail at startup.** Fatal error, exit non-zero. This
  matches the existing config-error pattern (better to fail loud than
  to start in a half-broken state).
- **All candidates fail at runtime.** Stay on the current active client,
  log ERROR every tick (rate-limited to once per minute), keep
  `state=probe_failed` in the status row. Subscription continues to
  use whatever the existing client can do; eventually one node recovers.
- **`signalSubscriptionRestart()` called rapidly.** Already coalesced
  by the size-1 buffered channel. No new safety needed.
- **Shutdown during failover swap.** `swapClient` checks `ctx.Done()`
  between steps. If cancelled mid-swap, the new client is closed
  immediately and the old client is not closed (Run returns and the
  process exits cleanly).

## Testing

### Unit tests

- `internal/indexer/watchdog_test.go`:
  - `classify()` table-driven tests for each state. Inputs:
    `(probe, dbHeight, lastProgressAt, now, thresholds)`. Verify
    expected classification.
  - Reaction dispatch: given classification + activeIdx, verify which
    side effects (restart signal, swap, status upsert) fire.
  - Asymmetric streak logic: simulate sequences of probes, verify
    failover triggers at `unhealthy_streak` and failback at
    `failback_streak`, not before.
- `internal/indexer/nodepool_test.go`:
  - `Probe` against a `httptest.Server` faking JSON-RPC responses.
  - Cached client per URL is reused across ticks.
- `internal/indexer/client_switch_test.go`:
  - Chain-id mismatch refuses swap.
  - Atomic publish: concurrent `Load()`s during swap return a valid
    client (never nil, never partially-initialised).
  - Old client closed after grace.
- `internal/repository/sync_status_test.go` (against the existing
  test database harness):
  - Single-row constraint enforced.
  - Upsert + Get round-trip.
  - Migrations apply and roll back cleanly.

### Integration tests (existing `-tags integration` harness)

- `internal/indexer/watchdog_integration_test.go` (with the test
  Postgres):
  - Boot indexer with a `httptest`-faked primary node that goes
    silent after N momentums. Verify watchdog flips to `stalled` →
    `signalSubscriptionRestart()` → recovers.
  - Boot with two fake nodes; primary returns `node_lagging` after
    M ticks. Verify failover to secondary, sync_status row updates,
    subscription resumes against secondary.
  - Then secondary stays healthy and primary recovers. Verify
    failback after `failback_streak` ticks, not before.
  - Chain-id mismatch case: secondary returns different chain identity (genesis hash).
    Verify swap refused, ERROR logged, no `activeClient` change.

### Smoke (manual)

- `docker compose --profile local-node up -d --build` with
  `NODE_URL_FALLBACKS=wss://my.hc1node.com:35998` in `.env`.
- `docker stop nom-indexer-znnd` to simulate local death.
  - Watch indexer logs: should see watchdog flag `node_lagging`,
    `probe_failed`, attempt failover, swap to remote, resume.
  - `curl :8080/healthz` reports degraded then ok-on-fallback.
  - `curl :9092/readyz` mirrors.
- `docker start nom-indexer-znnd`; wait `failback_streak * interval`.
  - Watchdog should swap back to local. Status row reflects.

## Rollout

- **Default disabled.** First merge ships with `watchdog.enabled: false`
  so existing deployments are unaffected. The migration is additive
  (just creates the new table); safe to apply.
- **Operator opt-in.** Flip to `true` in `config.yaml` or
  `INDEXER_WATCHDOG_ENABLED=true`.
- **Single-node deployments.** With only `nodes[0]` configured, the
  watchdog still does drift detection + `signalSubscriptionRestart()`;
  failover code paths are dead but cost nothing.
- **Existing `NODE_URL_WS`-only deployments.** Continue to work
  unchanged. The env var is read into `nodes[0].url` and watchdog
  defaults to disabled.

## Open questions / deferred

- Whether to expose the watchdog's classification as a Prometheus
  metric label rather than a sync_status column. Defer — both can be
  added; this design picks the table because it's queryable from API,
  MCP, and ad-hoc SQL without depending on Prometheus scraping.
- Per-node Prometheus latency histograms for `Probe`. Defer to a
  follow-up if operators want them.
- Backfill integration: if a long failover leaves a momentum gap in
  the DB (shouldn't happen — subscription's catch-up sync covers it,
  but if it did), should the watchdog kick `cmd/backfill`? Defer.
  The existing manual backfill is good enough until we see this in
  practice.

## References

- `internal/indexer/indexer.go:62, 84-86, 225-258, 260-314` — existing
  subscription + reconnect mechanism we're extending.
- `internal/indexer/indexer.go:547-565` — bridge sync loop pattern to
  model the watchdog goroutine after.
- `internal/indexer/cron.go:18-51` — multi-ticker pattern.
- `internal/indexer/retry.go:33-80` — `withRetry` exponential backoff
  used by all probe RPCs.
- `docs/architecture/overview.md` — process layout (where this new
  goroutine fits in the existing lane table).
- znnd `stats.syncInfo` RPC:
  <https://docs.0x3639.com/developer/rpc-api/core/stats#statssyncinfo>
