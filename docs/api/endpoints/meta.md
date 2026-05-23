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

Two checks: pings the Postgres pool **and** verifies the indexer
schema is present (looks for the `momentums` table). Returns
`200 {"status":"ready"}` when both pass, `503` with a problem+json
body when either fails. Safe for k8s readiness probes — it returns
not-ready until migrations have run on a fresh cluster.

```bash
curl -s http://localhost:8080/readyz
# {"status":"ready"}

# On a fresh DB before the indexer has migrated:
# {"type":"about:blank","title":"Service Unavailable","status":503,
#  "detail":"momentums table missing — start the indexer container so migrations run",
#  "code":"schema_not_migrated"}
```

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
