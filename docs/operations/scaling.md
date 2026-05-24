---
title: Scaling
---

# Scaling

This indexer is a single-process service writing to one Postgres
instance. Scaling means making that single pair go faster, not
horizontally fanning out. Below are the levers in priority order.

## Vertical: Postgres

The dominant cost of the indexer is Postgres writes. Going from a
laptop SSD to a server NVMe makes the biggest single difference.
Postgres 16's defaults are conservative; for a dedicated indexer DB
consider:

```ini
# postgresql.conf snippet
shared_buffers = 4GB              # ~25% of RAM
work_mem = 32MB
maintenance_work_mem = 256MB
wal_buffers = 64MB
checkpoint_timeout = 15min
max_wal_size = 4GB
synchronous_commit = on           # keep this on for durability
```

The compose-shipped Postgres uses defaults. Override via
`docker/postgres.conf` mounted as `command: -c
config_file=/etc/postgresql/postgresql.conf` if you want tuned config.

## Connection pool

`database.pool_size` (default 10) is the indexer's max pgxpool size.
Under steady-state load, the indexer typically holds 2–4 connections.
The `cmd/api` service opens its own pgxpool against the same DB
(its `database.pool_size` knob is independent). Raise either pool to
20–30 if you're running more replicas or expect heavy API traffic.

## Vertical: indexer

The indexer is largely single-threaded inside `processMomentum`. The
foreground sync/subscription lane runs alongside the bridge sync,
cached-data sync, and cron goroutines, plus the SDK's connection
lifecycle; all share the DB pool. Bottlenecks are typically:

- **Per-block `GetAccountBlockByHash` calls** to the node. Latency-bound.
- **`AcceleratorApi.GetAll` walk** during cached-data sync. Multi-page,
  multi-second.
- **Per-address `GetAccountInfoByAddress` calls** during balance refresh.

A faster local node (run alongside the indexer) cuts the round-trip
tax dramatically.

## Read replica (for API / MCP consumers)

The `cmd/api` and `cmd/mcp` services are pure read consumers — they
should not contend with the indexer's writes.
Two options:

1. **Postgres streaming replica.** Point read consumers at the replica.
   Lag is typically sub-second; fine for an explorer UI.
2. **Same primary, separate connection pool.** Simpler; works if total
   QPS stays modest. The indexer already runs at low transaction
   rates.

The default `docker-compose.yml` API service uses option 2 (same
primary). Switch to option 1 once contention becomes visible —
nothing in the API code path is replica-incompatible.

## Partitioning

`account_blocks` grows roughly with chain transaction count. At
3M+ rows it remains queryable; at 100M+ you may want to partition by
`momentum_height` ranges:

```sql
ALTER TABLE account_blocks RENAME TO account_blocks_old;
CREATE TABLE account_blocks (LIKE account_blocks_old INCLUDING ALL)
PARTITION BY RANGE (momentum_height);
-- … per-range child tables …
INSERT INTO account_blocks SELECT * FROM account_blocks_old;
```

This is a one-way door; do not undertake without a backup and an
extended maintenance window. The indexer code does not need changes —
inserts route to the right partition automatically.

## Horizontal: don't

Running two indexer processes against the same DB races on every
insert. Don't. If you need redundancy, run a standby Postgres replica
and fail over.

## Disk

| Chain age | Approx `./data` size |
|---|---|
| Genesis on test net | ~50 MB |
| 13M momentums (early 2026) | ~5 GB |
| Full mainnet projected (~30M momentums) | ~15–20 GB |

The `account_blocks` table dominates. The four `_stat_histories`
tables grow linearly with active dates × keys but stay tiny by
comparison.

## CPU and memory

The indexer's resident memory hovers around 50–100 MB. Postgres scales
with `shared_buffers` and the cached working set. A 4-core VM with
8 GB RAM is plenty for the indexer + Postgres combined on test net;
double that for production.

## Observability

The indexer's liveness signal is `MAX(momentums.timestamp)` advancing
— the indexer binary does not export Prometheus today. The
`cmd/api` service does, on a separate listener (`API_METRICS_PORT`,
default 9090), with request count + latency histograms labeled by
chi route pattern. See [`monitoring.md`](monitoring.md).
