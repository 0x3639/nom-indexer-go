# Accounts

`address` is the account's Zenon address (`z1q…`).

## Get account — `GET /api/v1/accounts/{address}`

Returns flow metrics (lifetime ZNN/QSR sent/received), delegation
state, and first/last activity timestamps. `404` if the indexer has
never observed the address.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/accounts/z1qq... | jq
```

## Balances — `GET /api/v1/accounts/{address}/balances`

Every `(token_standard, balance)` row for the address, sorted by
balance descending. Not paginated — accounts typically hold a
handful of tokens. Returns `{"data": []}` for an unknown address.

## Transactions — `GET /api/v1/accounts/{address}/transactions`

Paginated. Matches the address on either the sender (`address`) or
recipient (`to_address`) side, mirroring the repository's
`WHERE address = $1 OR to_address = $1` clause.

## Stakes / Fusions — `/stakes`, `/fusions`

See [Stakes & Fusions](stakes_fusions.md) for the address-scoped
variants. Both default to `is_active = true`; pass
`?include_inactive=true` to widen the set.

## Rewards — `/rewards`, `/rewards/cumulative`

See [Rewards](rewards.md).

## Bridge wraps / unwraps — `/bridge/wraps`, `/bridge/unwraps`

See [Bridge](bridge.md) for the address-scoped variants. Filters
on `to_address` (the destination of the cross-chain transfer).
