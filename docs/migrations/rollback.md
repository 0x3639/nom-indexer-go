---
title: Rollback discipline
---

# Rollback discipline

Down migrations are first-class. They exist for two reasons:

1. Developers re-create a clean schema during testing by stopping the
   stack, removing the bind-mounted `./data` directory, and starting the
   stack again.
2. Operators recover from a partial / dirty migration.

They are **not** typically used for production rollbacks against
populated databases, because most schema changes that drop columns or
tables are inherently lossy.

## What's safely reversible

- `CREATE TABLE` → `DROP TABLE`. Loses data; only OK in dev.
- `CREATE INDEX` → `DROP INDEX`. Lossless.
- `ALTER TABLE … ADD COLUMN` → `ALTER TABLE … DROP COLUMN`. Loses the
  column's data.
- `CREATE UNIQUE INDEX` after data dedup → drop the constraint.

## What's *not* safely reversible

- Renaming a column. Both up and down rename — that's fine — but if any
  query in code references the column it must be coordinated with a
  code change.
- Data backfills inside the migration. (Don't do this — use a `scripts/`
  one-shot instead. See [`guide.md`](guide.md#authoring-rules).)
- Dropping a NOT NULL constraint when production data has nulls.

## Recovering from a dirty migration

`golang-migrate` marks the `schema_migrations` table dirty when an up
or down partially applied. The indexer logs this on startup. To recover:

```bash
# 1. Identify the dirty version.
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "SELECT version, dirty FROM schema_migrations;"

# 2. Hand-apply any incomplete portion, or hand-undo if the up didn't
#    complete and you want to retry.
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "..."

# 3. Clear the dirty flag.
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c "UPDATE schema_migrations SET dirty = false;"

# 4. Restart the indexer; it will re-run the up.
docker compose restart indexer
```

## Forced version

If the migrator can't figure out where it is, force-set the version:

```sql
UPDATE schema_migrations SET version = <n>, dirty = false;
```

Use sparingly — it's the equivalent of "trust me, the DB is at version
N". Use only after manually verifying the DB schema matches that
version.

## Production rollback playbook

For a production rollback of a release that introduced a bad migration:

1. **Stop writes.** Take the indexer down (`docker compose stop
   indexer`); the database stays running.
2. **Take a backup.** `scripts/backup.sh` — see
   [`docs/operations/backup-restore.md`](../operations/backup-restore.md).
3. **Apply the down migration manually.** Read the `*.down.sql` and run
   the statements with the integrator's understanding of what they
   destroy.
4. **Restart on the previous release.** The image tag of the previous
   release should match the schema version after the down.

For lossy downs (column drops, table drops), prefer a forward-fix
release that re-creates the column with a default rather than rolling
back.

## Re-creating the DB from scratch

In dev:

```bash
docker compose down           # stops containers; ./data remains
rm -rf ./data                 # destructive: removes the Postgres bind mount
docker compose up -d          # fresh DB, runs all migrations
```

In production: **never**. Use a forward-fix release.
