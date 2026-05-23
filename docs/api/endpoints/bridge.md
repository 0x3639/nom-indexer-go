# Bridge

Cross-chain wrap/unwrap requests handled by the Zenon bridge.

## Wraps — Zenon → external chain

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/bridge/wraps | jq

# Wraps with a specific destination address
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/0xabc.../bridge/wraps | jq
```

| Route | Notes |
|---|---|
| `GET /api/v1/bridge/wraps` | Paginated; ordered by `creation_momentum_height DESC`. |
| `GET /api/v1/accounts/{address}/bridge/wraps` | Filters on `to_address` (the external destination). |

## Unwraps — external chain → Zenon

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/bridge/unwraps | jq

curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/z1qq.../bridge/unwraps | jq
```

| Route | Notes |
|---|---|
| `GET /api/v1/bridge/unwraps` | Paginated; ordered by `registration_momentum_height DESC`. |
| `GET /api/v1/accounts/{address}/bridge/unwraps` | Filters on `to_address` (the Zenon destination). |

## Finalization state

A wrap is finalized when `confirmations_to_finality = 0`. An unwrap
is finalized when `redeemed = true` or `revoked = true`. The
indexer keeps both states fresh as the bridge orchestrator
progresses.
