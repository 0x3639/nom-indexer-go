# Generated column reference

This page is a placeholder for the optional `tbls`-generated schema
appendix. The checked-in hand-written pages in `docs/schema/` are the
authoritative table reference today.

To regenerate the appendix from a migrated test database:

```bash
TEST_DATABASE_URL='postgres://postgres:<pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  scripts/docs/gen-schema-tbls.sh
```

The script requires `tbls` in `PATH` and overwrites this file with the
database-derived column and index listing.
