# Pillars

Pillars are Zenon's validator nodes.

## List — `GET /api/v1/pillars`

Ordered by `rank ASC`. Excludes revoked pillars by default.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/pillars?page=1&page_size=50' | jq

# Include revoked pillars
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/pillars?include_revoked=true' | jq
```

## By name — `GET /api/v1/pillars/{name}`

Pillar names are unique in Zenon.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/pillars/alphanet-1 | jq
```

## Delegators — `GET /api/v1/pillars/{name}/delegators`

Looks up the pillar by name first, then queries `accounts.delegate`.
Sorted by `delegation_start_timestamp ASC` (longest-tenured first).
Returns `404` if the pillar name is unknown.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/pillars/alphanet-1/delegators?page=1&page_size=20' | jq
```
