# Stakes & Fusions

Both default to `is_active = true`; pass `?include_inactive=true`
to include cancelled / expired entries.

## Stakes

ZNN staked for delegation/voting weight.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/stakes | jq

curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/z1qq.../stakes | jq
```

| Route | Notes |
|---|---|
| `GET /api/v1/stakes` | Paginated list, all addresses. |
| `GET /api/v1/accounts/{address}/stakes` | Paginated, filtered to one address. |

## Fusions

QSR fused to provide plasma (compute) for an address.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/fusions | jq

curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/z1qq.../fusions | jq
```

| Route | Notes |
|---|---|
| `GET /api/v1/fusions` | Paginated list, all entries. |
| `GET /api/v1/accounts/{address}/fusions` | Paginated; address matches the funder OR beneficiary side. |
