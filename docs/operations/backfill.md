---
title: Backfill
---

# Backfill

Three paths exist for filling gaps in the indexer's tables. Pick by use case.

## 1. `BACKFILL_ON_STARTUP=true`

Set this env var (in `.env` or compose) and restart the indexer. The
binary will scan for gaps in `momentums` and incomplete momentums
(where `tx_count > 0` but the matching account blocks are missing),
then process them before entering real-time sync.

```bash
echo 'BACKFILL_ON_STARTUP=true' >> .env
docker compose up -d
docker logs nom-indexer -f
```

Use when: a fresh deploy onto a populated DB that was left behind by
an older indexer release.

Cost: scales with gap count. ~40 momentums/sec on a healthy
local + remote stack. 10k gaps ≈ 4 minutes.

Once done, remove or set `BACKFILL_ON_STARTUP=false` so subsequent
restarts don't redo the scan.

## 2. `cmd/backfill` standalone tool

The standalone binary at
[`cmd/backfill/main.go`](https://github.com/0x3639/nom-indexer-go/blob/main/cmd/backfill/main.go)
performs the same gap-fill without restarting the live indexer.

```bash
DATABASE_PASSWORD=<pw> DATABASE_ADDRESS=localhost \
  GOWORK=off go run ./cmd/backfill 2>&1 | tee backfill.log
```

Use when: you want to fill gaps **while** the indexer keeps syncing
new blocks. Both processes share the DB; conflicting inserts use the
same `ON CONFLICT (height) DO NOTHING` so neither corrupts the other.

The tool delegates to `indexer.Backfill` so the gap-finding query and
processing path are shared with `BACKFILL_ON_STARTUP`.

## 3. One-shot scripts for specific data

The [`scripts/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts)
directory carries focused tools for data the indexer can't backfill by
re-running:

| Script | Purpose |
|---|---|
| [`scripts/backfill-rewards/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards) | Re-derives `reward_transactions` + `cumulative_rewards` from `account_blocks` using the corrected BlockType pattern. Idempotent. Run once on a DB that has pre-fix history. |
| [`scripts/repair-votes/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/repair-votes) | Re-decodes votes from `account_blocks.data` using a known-good SDK. Drops + reinserts. |
| [`scripts/phase-outreach/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/phase-outreach) | Operational query helper, not strictly a backfill. |

Each script reads the same env vars as the indexer
(`DATABASE_PASSWORD`, etc.); see the script's source for any
script-specific args.

## Verifying gap closure

After any of the above, sanity-check:

```sql
SELECT MIN(height), MAX(height), COUNT(*),
       MAX(height) - MIN(height) + 1 - COUNT(*) AS gaps
FROM momentums;
```

`gaps = 0` means no missing momentums. The "incomplete momentum" check
(below) catches momentums whose `tx_count` exceeds the number of
account blocks stored:

```sql
SELECT COUNT(*) FROM momentums m
LEFT JOIN (
  SELECT momentum_height, COUNT(*) AS actual_txs
  FROM account_blocks GROUP BY momentum_height
) ab ON m.height = ab.momentum_height
WHERE m.tx_count > 0 AND COALESCE(ab.actual_txs, 0) < m.tx_count;
```

## What backfill *won't* do

- Re-process tables that have no source-of-truth in `account_blocks`
  (`pillars`, `sentinels`, `tokens` non-event fields). These come from
  the cached-data sync; restart the indexer to refresh them.
- Repopulate `balances` for past heights. Balance updates skip
  high-tx-count momentums by design (see
  [`schema/balances.md`](../schema/balances.md)).
- Fix data that's downstream of a known bug (e.g., pre-classification
  reward type splits). Those need a dedicated script —
  `backfill-rewards` is the prototype to copy.
