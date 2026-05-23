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

Pings the Postgres pool. Returns `200` `{"status":"ready"}` when
the DB is reachable, `503` with a problem+json body when it is not.

```bash
curl -s http://localhost:8080/readyz
# {"status":"ready"}
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
