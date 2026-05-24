# Momentums

A momentum is a block header in Zenon's dual-ledger model. See the
[schema reference](../../schema/index.md) for column-level meaning.

## List — `GET /api/v1/momentums`

Paginated. Sort by `height`; default `desc` (newest first).

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/momentums?page=1&page_size=10' | jq
```

## Latest — `GET /api/v1/momentums/latest`

The highest-height momentum. Returns `404` if the indexer has not
processed any blocks yet.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/momentums/latest | jq
```

## By height — `GET /api/v1/momentums/{height}`

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/momentums/12345 | jq
```

| Path param | Type | Notes |
|---|---|---|
| `height` | uint64 | Non-numeric / negative input → `400 invalid_height`. Unknown height → `404`. |
