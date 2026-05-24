# Tokens

ZTS tokens registered on the Zenon Network.

## List — `GET /api/v1/tokens`

Ordered by `holder_count DESC` then `token_standard ASC`
(most-held first; deterministic ties). Paginated. Sort param is
intentionally not honored — the order is fixed.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/tokens?page=1&page_size=20' | jq
```

## By standard — `GET /api/v1/tokens/{token_standard}`

```bash
# ZNN token
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/tokens/zts1znnxxxxxxxxxxxxx9z4ulx | jq
```

## Holders — `GET /api/v1/tokens/{token_standard}/holders`

Paginated richlist. Sorted by balance DESC. Excludes zero balances
via the existing partial index.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/tokens/zts1znnxxxxxxxxxxxxx9z4ulx/holders?page=1&page_size=10' | jq
```
