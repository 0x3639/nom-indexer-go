---
title: Sync watchdog
---

# Sync watchdog

The watchdog detects two failure modes and reacts in-process so the
indexer container does not need to restart:

1. **Indexer behind znnd.** Triggers an internal subscription restart
   (the same path the SDK reconnect callback uses), which re-runs the
   batched catch-up sync.
2. **znnd behind the chain** (and/or fully unresponsive). Triggers a
   failover to the next configured node.

The default behaviour is **off** — operators opt in by enabling the
watchdog in config or via the `INDEXER_WATCHDOG_ENABLED` env var. With
the watchdog enabled but no fallback nodes configured, drift detection
and subscription restart still work; failover is a no-op until at
least one fallback is configured.

## Configuration

See `config.yaml.example` for the full set. Minimum to enable
with a fallback node:

```yaml
indexer:
  nodes:
    - url: ws://znnd:35998
      label: local-znnd
    - url: wss://my.hc1node.com:35998
      label: hc1node-remote
  watchdog:
    enabled: true
```

Via env:

- `INDEXER_WATCHDOG_ENABLED=true`
- `NODE_URL_FALLBACKS=wss://my.hc1node.com:35998`
  (comma-separated for multiple)

The full watchdog tuning knobs (interval, thresholds, streaks) live
under `indexer.watchdog` in `config.yaml.example`. Defaults are
production-safe; only adjust if you have a specific drift-recovery
target in mind.

## Health endpoints

The indexer container exposes:

| Endpoint | Body shape | Meaning |
|---|---|---|
| `:9092/healthz` | `{"status":"ok"}` | Process alive (always 200). |
| `:9092/readyz` | `{"status":"ready", "node":"label", "drift":N, "state":"synced"}` (200) or `{"status":"draining", "state":"node_lagging", ...}` (503) | Reflects the watchdog's last classification. 503 when state ≠ synced for ≥ 2 consecutive bad ticks. |

The API process's existing `/readyz` ALSO consults the
`indexer_sync_status` row, so external monitoring of API health surfaces
indexer-side degradation:

- 200 ok if the indexer is synced.
- 503 `indexer_drift` if the indexer has drifted past the streak
  threshold.
- 200 `{"status":"ready","watchdog":"inactive","watchdog_stale_seconds":N}`
  when the row hasn't been written for > 5 minutes. That row is written
  only by the watchdog loop, so a stale `checked_at` means the watchdog is
  **disabled or stopped** — its `node`/`drift`/`state` are frozen and would
  be misleading, so `/readyz` reports them as inactive rather than serving
  the stale values. (Expected whenever `indexer.watchdog.enabled: false`,
  e.g. during an initial cold sync — use the indexer's `/api/v1/status`
  `latest_height` for live progress in that mode.)

Docker compose has a healthcheck wired to the indexer `/healthz`; this
keeps the container restart policy informed of process liveness without
restarting the indexer when the indexer is healthy-but-degraded
(the right behavior, since restarting wouldn't fix a znnd lag).

## Observing state

The watchdog upserts a single row in `indexer_sync_status` each tick:

```sql
SELECT state, drift_momentums, node_lag_momentums,
       active_node_label, consecutive_bad_checks,
       to_timestamp(checked_at) AS checked
  FROM indexer_sync_status;
```

States:

- `synced` — caught up to znnd, znnd caught up to chain.
- `indexer_lagging` — DB behind znnd by more than `indexer_drift_threshold`.
- `node_lagging` — znnd behind chain by more than `node_drift_threshold`.
- `stalled` — no momentum has been committed for longer than `stall_threshold`.
- `probe_failed` — the watchdog's `stats.syncInfo` probe failed; everything else is unknown until the next tick.

## Manual smoke

To exercise the failover/failback paths on a running stack with the
local-node profile:

```bash
# Bring up with watchdog enabled, primary on local znnd, fallback on
# a remote.
docker compose --profile local-node up -d --build

# Simulate local failure:
docker stop nom-indexer-znnd
# Wait ~90s (2 watchdog ticks at 30s + swap latency).
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c 'SELECT state, active_node_label, failed_over_at FROM indexer_sync_status;'

# Restore primary; failback after ~2.5 minutes (5 ticks of healthy).
docker start nom-indexer-znnd
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c 'SELECT state, active_node_label, failed_over_at FROM indexer_sync_status;'
```

## Known issues

### False-positive failover during initial sync or backfill

`lastProgressAt` is seeded to the indexer's start time, but the watchdog
loop launches before initial `sync()` returns. If catch-up takes longer
than `stall_threshold` (default 60s) — for example a fresh deployment
with a large gap, or `BACKFILL_ON_STARTUP=true` — the watchdog may
classify `stalled` and after `unhealthy_streak` ticks fail over to the
next configured node even though the indexer is healthy.

This transient false failover **no longer strands the indexer.** Failback
is allowed whenever the active node is healthy and only the indexer is
behind (`indexer_lagging`), not just when fully `synced`, and only to a
primary that is itself at head. So after a false failover the watchdog
returns to a recovered primary within ~`failback_streak × interval`
(~2.5 min by default) — even mid-cold-sync. (Previously, failback was
gated on `synced`, which never occurs during a multi-hour catch-up, so the
indexer was stranded on the fallback for the entire sync.)

To avoid the transient failover churn entirely during a large first sync:
- Keep `indexer.watchdog.enabled: false` during the first run; flip to
  `true` after the indexer has caught up. (Cleanest.)
- Or raise `indexer.watchdog.stall_threshold` to comfortably exceed the
  worst-case initial-sync stall.
- Configure the fallback node identical to the primary so a false
  failover doesn't change runtime behaviour.

Remaining follow-up: suppress the false `stalled` classification during an
active catch-up so the initial failover doesn't happen in the first place.

