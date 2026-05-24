# Meta endpoints

Liveness, readiness, and indexer sync state.

## Liveness — `GET /healthz`

Always returns `200 OK` if the process is up. Suitable for k8s
liveness probes. Does **not** check the database.

```bash
curl -s http://localhost:8080/healthz
# {"status":"ok"}
```

## Readiness — `GET /readyz`

Two checks:

1. Pings the Postgres pool.
2. Reads golang-migrate's `schema_migrations` and asserts
   `version >= minSchemaVersion` (currently `11`) AND `dirty = false`.

Returns `200 {"status":"ready"}` when both pass. Returns `503` with a
problem+json body on any failure mode below. Safe for k8s readiness
probes — `/readyz` reports not-ready on a fresh cluster, a partially
migrated DB, or a migration that crashed mid-run.

```bash
curl -s http://localhost:8080/readyz
# {"status":"ready"}
```

Failure shapes:

| `code` | Status | When |
|---|---|---|
| `db_unavailable` | 503 | Pool ping or query failed (network, pool exhausted). |
| `schema_not_migrated` | 503 | `schema_migrations` missing, or its `version` < `minSchemaVersion`. Start the indexer container so migrations run. |
| `schema_dirty` | 503 | `schema_migrations.dirty = true`. The last migration crashed mid-run; manual repair required. |

`minSchemaVersion` is pinned in `internal/api/router/router.go` and
must be bumped whenever a new migration touches a table the API
reads. The constant is intentionally separate from the indexer's
migration tooling so a deploy that ships the API without the matching
migration cleanly reports `schema_not_migrated` instead of 500ing on
the first request.

## Indexer sync state — `GET /api/v1/status`

Returns the latest indexed momentum height, its timestamp, and the
current indexer lag (server clock minus the latest momentum's
timestamp). All derived from the database — no node round-trip.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/status | jq
```

```json
{
  "latest_height": 12345,
  "latest_timestamp": 1700000000,
  "indexer_lag_seconds": 5,
  "version": "v1.0.0"
}
```

`indexer_lag_seconds > 10` typically indicates the indexer is
falling behind the chain head.
