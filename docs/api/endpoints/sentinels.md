# Sentinels

Sentinels are Zenon's secondary network nodes (lighter-stake than
pillars). Paginated; default sorted by `registration_timestamp DESC`
(newest first).

## List — `GET /api/v1/sentinels`

```bash
# Active sentinels only (default)
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/sentinels | jq

# Include retired sentinels
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/sentinels?include_inactive=true' | jq
```
