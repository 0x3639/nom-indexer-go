# Account blocks (transactions)

Account-blocks are Zenon's dual-ledger transactions: one block is
either a send (`block_type` 1–4) or a receive (5–8); a complete
transfer is the matched (send, receive) pair.

## List — `GET /api/v1/account_blocks`

Paginated. Sort by `momentum_height`; default `desc`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/account_blocks?page=1&page_size=20' | jq
```

## By hash — `GET /api/v1/account_blocks/{hash}`

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/account_blocks/<block-hash> | jq
```

## Transactions for an address

`GET /api/v1/accounts/{address}/transactions` — see [Accounts](accounts.md).

## On the `amount` field

`amount` is a JSON string (see [API conventions](../index.md#amounts-are-json-strings)).
A 1 ZNN transfer is represented as `"100000000"` (8 decimals).
