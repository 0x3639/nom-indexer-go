---
title: Backup and restore
---

# Backup and restore

Two shell scripts ship with the repo:
[`scripts/backup.sh`](https://github.com/0x3639/nom-indexer-go/blob/main/scripts/backup.sh)
and
[`scripts/restore.sh`](https://github.com/0x3639/nom-indexer-go/blob/main/scripts/restore.sh).
Both target the `nom-indexer-postgres` container.

## Backup

```bash
# Create a timestamped, compressed backup in ./backups/
./scripts/backup.sh

# Or specify the output path:
./scripts/backup.sh /path/to/my-backup.sql.gz
```

What it does:

1. Checks that `nom-indexer-postgres` is running.
2. `docker exec nom-indexer-postgres pg_dump -U postgres nom_indexer | gzip > <out>`.
3. Reports the resulting file size.

The output is a standard gzipped `pg_dump` SQL dump.

## Restore

```bash
./scripts/restore.sh ./backups/nom_indexer_20260606_120000.sql.gz
```

What it does:

1. Confirms the backup file exists.
2. **Drops and recreates** the `nom_indexer` database (asks for
   confirmation first).
3. Pipes the (optionally gzipped) dump back through `psql`.
4. Reports success.

The script supports both `.sql` and `.sql.gz`.

## Retention

The scripts don't manage retention — `./backups/` is gitignored but
otherwise untouched. Add your own cron if you want time-based
rotation:

```bash
find ./backups -name '*.sql.gz' -mtime +30 -delete
```

## Disaster recovery drill

Once per quarter:

```bash
# 1. Make a backup of the live DB.
./scripts/backup.sh ./backups/dr-drill.sql.gz

# 2. Create a fresh DB for the test.
docker exec nom-indexer-postgres psql -U postgres -c 'CREATE DATABASE dr_test;'

# 3. Restore into it.
gunzip -c ./backups/dr-drill.sql.gz | \
  docker exec -i nom-indexer-postgres psql -U postgres -d dr_test

# 4. Verify row counts match.
docker exec nom-indexer-postgres psql -U postgres -d dr_test \
  -c "SELECT (SELECT COUNT(*) FROM momentums) AS m_count;"
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "SELECT (SELECT COUNT(*) FROM momentums) AS m_count;"

# 5. Clean up.
docker exec nom-indexer-postgres psql -U postgres -c 'DROP DATABASE dr_test;'
```

If row counts disagree, the backup is incomplete — investigate
`pg_dump`'s exit code and re-run.

## Restoring into a different host

Copy the backup file, install Postgres 16 on the target, and:

```bash
gunzip -c backup.sql.gz | psql -U postgres -d <target_db>
```

The script calls plain `pg_dump` without `--no-owner`, so ownership
statements may be present in the dump. Restoring as the same `postgres`
role used by the compose stack is the least surprising path. If you
restore into a differently owned database, add the appropriate
`pg_dump` flags or edit the dump for that environment.

## Backups vs replication

The scripts produce **point-in-time dumps**, not a hot standby. For
production HA you want either:

- Postgres streaming replication (out of scope for this repo).
- A continuous archiver like WAL-E / WAL-G.

The dump-and-restore flow above is sufficient for daily backups and
disaster-recovery drills.

## Migrations after restore

A restored database carries its own `schema_migrations` row. The
restored indexer's startup `m.Up()` will see no pending migrations
and proceed.

If you restore an older dump and upgrade the binary in the same pass,
the binary's startup will run the pending migrations. The dump's row
counts won't include data the new migrations would have populated;
that's expected.
