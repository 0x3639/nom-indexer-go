---
title: Monitoring
---

# Monitoring

What "healthy" looks like and how to spot trouble.

## Sync progress

The canonical health signal is "is the sync cursor advancing?". One
query:

```sql
SELECT MAX(height) AS db_head,
       (extract(epoch from now()) - MAX(timestamp))::int AS seconds_behind
FROM momentums;
```

In real-time mode the indexer is processing roughly one momentum every
10 seconds (the chain's target block time), so `seconds_behind < 30`
means caught up.

A healthy log around the frontier:

```
INFO  indexer/indexer.go  initial sync complete, starting real-time subscription
INFO  indexer/indexer.go  subscribed to momentums
INFO  indexer/indexer.go  received new momentum  {"height": <h>}
```

## During initial sync

Initial sync (from genesis or after long downtime) processes batches of
100 momentums at a time and logs each. Throughput is roughly 50
momentums/sec on a healthy local Postgres + remote node. For 13M
momentums that's ~3 days; running the indexer locally next to a node
brings it under a day.

To watch live throughput:

```bash
docker logs nom-indexer -f 2>&1 | grep "processing momentum" | tail -f
```

## Gap detection

Run periodically (or wire into Prometheus):

```sql
SELECT MAX(height) - MIN(height) + 1 - COUNT(*) AS gaps
FROM momentums;
```

If `gaps > 0`, see [`backfill.md`](backfill.md).

## Reward indexing health

Reward indexing was historically broken (see
[`docs/reference/known-issues.md`](../reference/known-issues.md)). Spot
check:

```sql
SELECT reward_type, COUNT(*)
FROM reward_transactions
GROUP BY reward_type
ORDER BY reward_type;
```

You should see rows for types 1 (Stake), 3 (Liquidity), 4 (Sentinel),
5 (Pillar), and post-classification fix also 2 (Delegation). Empty
results past type 3 means either a chain with no recent rewards or a
broken indexer — check the recent timestamp:

```sql
SELECT MAX(momentum_timestamp), COUNT(*) FROM reward_transactions;
```

## Bridge sync health

```sql
-- Pending wraps:
SELECT COUNT(*) FROM wrap_token_requests WHERE confirmations_to_finality > 0;

-- Pending unwraps:
SELECT COUNT(*) FROM unwrap_token_requests WHERE NOT redeemed AND NOT revoked;

-- Bridge config refreshed within last 5 min:
SELECT last_updated_timestamp,
       extract(epoch from now()) - last_updated_timestamp AS age_seconds
FROM bridge_admin WHERE row_id = 1;
```

A `bridge_admin.last_updated_timestamp` older than ~120 seconds
indicates the bridge sync is stuck. Check logs for
`bridge sync: failed` lines.

## Logs

The indexer uses `zap` structured logs. Useful fields:

| Field | Meaning |
|---|---|
| `height` | Momentum being processed. |
| `txCount` | Number of account blocks in the momentum. |
| `duration` | `processMomentum` wall time. |
| `op` | Set on retry warnings (e.g., `"GetMomentumsByHeight"`). |
| `attempt` | Retry attempt counter. |
| `backoff` | Sleep before the next retry. |

Filter for non-INFO lines:

```bash
docker logs nom-indexer 2>&1 | grep -vE 'INFO|MERMAID' | tail -50
```

## Common alert candidates

| Symptom | Alert | Action |
|---|---|---|
| `MAX(momentums.timestamp) < now() - 300s` | Sync stalled. | See [`failure-modes.md`](failure-modes.md). |
| Repeated "transient error, retrying" WARN with `op = GetFrontierMomentum` | Node unreachable. | Check `NODE_URL_WS`. |
| Repeated "bridge sync: failed" WARN | Bridge RPC broken. | Check the node's bridge support. |
| `gaps > 0` after a steady-state window | Backfill needed. | See [`backfill.md`](backfill.md). |
| Reward tables empty for a recent day | Reward indexing broken or no rewards. | Spot-check the receive paths. |

## Prometheus / metrics

The indexer binary does not expose Prometheus metrics today —
operators read sync state from Postgres (see the canonical liveness
query above).

The `cmd/api` HTTP service does ship Prometheus metrics on a
separate listener (port 9090 by default) exposing
`nom_api_http_requests_total` and `nom_api_http_request_duration_seconds`
labeled by method/route/status. See the
[API overview](../api/index.md). The route label uses the chi
template (e.g. `/api/v1/momentums/{height}`) so label cardinality
stays bounded.
