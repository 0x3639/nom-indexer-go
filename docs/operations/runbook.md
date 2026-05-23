---
title: Incident runbook
---

# Incident runbook

Numbered playbooks for the most common incidents. Each is a tight
sequence of commands you can paste under pressure.

## 1. Sync stuck (`MAX(timestamp)` not advancing)

**Symptom:** Sync cursor isn't moving for >60 seconds in steady
state.

```bash
# 1. Confirm the indexer is alive.
docker compose ps

# 2. Check the indexer's recent logs.
docker logs nom-indexer --tail 100

# 3. If the indexer's running, peek at the sync cursor.
docker exec nom-indexer-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT MAX(height), extract(epoch from now()) - MAX(timestamp) AS seconds_behind FROM momentums;"

# 4. If the node is the problem, try a different one.
echo 'NODE_URL_WS=wss://your-backup-node.example.com' >> .env
docker compose up -d indexer

# 5. If the indexer is stuck mid-batch, restart it.
docker compose restart indexer
```

See [`failure-modes.md`](failure-modes.md#subscription-stall).

## 2. Database full

**Symptom:** Postgres logs `could not extend file: No space left on device`.

```bash
# 1. Stop the indexer (Postgres can stay up).
docker compose stop indexer

# 2. Free space — old backups + Dagger engine.
ls -lh ./backups/
rm ./backups/<old>.sql.gz
docker stop dagger-engine-* 2>/dev/null; docker volume prune -f

# 3. Confirm space is back.
df -h ./data

# 4. Restart.
docker compose up -d indexer
```

## 3. Gap in momentums after restart

**Symptom:** `gaps > 0` from the monitoring query.

```bash
# Option A: in-place backfill on next restart (slower start).
echo 'BACKFILL_ON_STARTUP=true' >> .env
docker compose up -d indexer

# Option B: standalone tool while live sync continues.
DATABASE_PASSWORD=<pw> DATABASE_ADDRESS=localhost \
  GOWORK=off go run ./cmd/backfill 2>&1 | tee backfill.log
```

See [`backfill.md`](backfill.md).

## 4. Rewards missing or wrong

**Symptom:** `reward_transactions` empty or only has liquidity rewards.

```bash
# 1. Check current state.
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "SELECT reward_type, COUNT(*) FROM reward_transactions GROUP BY reward_type;"

# 2. If empty or pre-fix, run the backfill.
DATABASE_PASSWORD=<pw> DATABASE_ADDRESS=localhost \
  GOWORK=off go run ./scripts/backfill-rewards 2>&1 | tee rewards.log
```

The script is idempotent — re-running is safe.

## 5. Vote counts wrong

**Symptom:** A pillar's votes look stale or wrong.

```bash
# Re-decode votes from account_blocks using the known-good SDK.
DATABASE_PASSWORD=<pw> DATABASE_ADDRESS=localhost \
  GOWORK=off go run ./scripts/repair-votes 2>&1 | tee votes.log
```

The script drops + re-inserts; takes a few minutes on a busy DB.

## 6. Bridge sync failing

**Symptom:** Repeated `bridge sync: failed` log lines.

```bash
# 1. Inspect the error.
docker logs nom-indexer --since 10m 2>&1 | grep "bridge sync"

# 2. If it's "unknown network", that's the known SDK / test-net quirk
#    — log noise only, see failure-modes.md.

# 3. If it's an RPC timeout, check node health and swap NODE_URL_WS
#    if needed.

# 4. Check when the bridge_admin row was last refreshed:
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "SELECT last_updated_timestamp, extract(epoch from now()) - last_updated_timestamp AS age FROM bridge_admin WHERE row_id = 1;"
```

## 7. Restore from backup

See [`backup-restore.md`](backup-restore.md). Short form:

```bash
docker compose stop indexer
./scripts/restore.sh ./backups/<backup>.sql.gz   # destructive — asks to confirm
docker compose start indexer
```

## 8. Dagger CI runs failing on disk pressure

```bash
docker stop dagger-engine-* && docker rm dagger-engine-* && docker volume prune -f
```

## 9. Indexer panicking in cached-data sync

**Symptom:** Container restarts in a loop; logs show a panic mentioning
`accelerator_types` or a JSON unmarshal error.

```bash
# Confirm which SDK version is built into this checkout.
grep 'github.com/0x3639/znn-sdk-go' go.mod

# If the panic appeared after a bump, roll back or pin the SDK version,
# then rebuild.
docker compose up -d --build indexer
```

See [`docs/reference/known-issues.md`](../reference/known-issues.md).

## 10. The whole stack needs to come down cleanly

```bash
docker compose down       # stops both containers, keeps ./data
# To also nuke the DB bind mount:
docker compose down
rm -rf ./data             # destructive — gone
```
