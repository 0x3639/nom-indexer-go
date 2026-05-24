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

`pagination.total` here is **approximate** — sourced from
`pg_class.reltuples`, which Postgres refreshes via ANALYZE /
autovacuum. Expect drift on the order of the autovacuum cadence.
The scoped variant (`/api/v1/accounts/{address}/transactions`)
returns an exact count. See
[Pagination → Approximate totals](../pagination.md#approximate-totals-on-large-list-endpoints).

## By hash — `GET /api/v1/account_blocks/{hash}`

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/account_blocks/<block-hash> | jq
```

## Transactions for an address

`GET /api/v1/accounts/{address}/transactions` — see [Accounts](accounts.md).

## Stream — `GET /api/v1/transactions/stream` (WebSocket)

Live stream of newly-indexed account_blocks. Same plumbing as
[`/api/v1/momentums/stream`](momentums.md):
LISTEN/NOTIFY, JWT via header or `?token=`, per-subject cap,
`?from_height=` replay.

### Filtering

The transactions stream adds an optional `?address=` query parameter
that server-side filters frames to blocks where `address` (sender) OR
`to_address` (recipient) matches. Cheap (one string compare per
dispatched frame) and much smaller bandwidth than the firehose +
client-side filter.

```bash
# Firehose — every account_block the indexer commits.
wscat -c "ws://localhost:8080/api/v1/transactions/stream" \
      -H "Authorization: Bearer $TOKEN"

# Filtered — only blocks touching this address.
wscat -c "ws://localhost:8080/api/v1/transactions/stream?address=z1qq..." \
      -H "Authorization: Bearer $TOKEN"
```

### Replay

`?from_height=N` backfills account_blocks starting at MOMENTUM height
`N` (not account-block id) up to the chain tip via a single range
scan, then switches to live. Capped at 10,000 rows; for larger
historical windows, use the REST `/api/v1/account_blocks` endpoint.

### Browser

```javascript
const url = `ws://localhost:8080/api/v1/transactions/stream` +
            `?token=${encodeURIComponent(jwt)}` +
            `&address=${myAddress}` +    // optional
            `&from_height=${lastSeen}`;  // optional
const ws = new WebSocket(url);
ws.onmessage = e => {
  const block = JSON.parse(e.data);
  console.log(block.hash, block.address, '→', block.to_address, block.amount);
};
```

## On the `amount` field

`amount` is a JSON string (see [API conventions](../index.md#amounts-are-json-strings)).
A 1 ZNN transfer is represented as `"100000000"` (8 decimals).
