# Rewards

Per-account reward surfaces.

## Per-event history — `GET /api/v1/accounts/{address}/rewards`

Paginated. Returns each reward-receive transaction the indexer
classified for the address, ordered by `momentum_height DESC`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/accounts/z1qq.../rewards?page=1&page_size=50' | jq
```

## Cumulative totals — `GET /api/v1/accounts/{address}/rewards/cumulative`

Not paginated. Returns one row per `(reward_type, token_standard)`
bucket — the lifetime sum the indexer rolled up.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/z1qq.../rewards/cumulative | jq
```

`reward_type` values: `Pillar`, `Sentinel`, `Stake`, `Delegation`,
`Liquidity`, `Unknown`.
