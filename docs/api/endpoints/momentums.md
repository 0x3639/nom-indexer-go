# Momentums

A momentum is a block header in Zenon's dual-ledger model. See the
[schema reference](../../schema/index.md) for column-level meaning.

## List — `GET /api/v1/momentums`

Paginated. Sort by `height`; default `desc` (newest first).

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/momentums?page=1&page_size=10' | jq
```

`pagination.total` is an upper bound derived from `MAX(height)` —
exact when the chain has no backfill gaps. See
[Pagination → Approximate totals](../pagination.md#approximate-totals-on-large-list-endpoints).

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

## Stream — `GET /api/v1/momentums/stream` (WebSocket)

Upgrades to a WebSocket and pushes one JSON frame per newly-indexed
momentum (same shape as the REST endpoints). Powered by Postgres
`LISTEN`/`NOTIFY` — no polling, sub-second latency, one persistent
connection per API process regardless of subscriber count.

### Auth

Either of:

- `Authorization: Bearer <token>` header (CLI / server-side).
- `?token=<jwt>` query parameter (browsers — the WebSocket API
  cannot send custom headers on the upgrade request). Tokens passed
  this way may end up in HTTP proxy logs; mint short-TTL tokens for
  stream consumers.

### Replay

Pass `?from_height=N` to backfill momentums starting at height `N`
before switching to live streaming. The catch-up scan is capped at
10,000 rows — beyond that, use the REST `/api/v1/momentums` endpoint
for the historical gap and reconnect for live.

### Close codes

| Code | Meaning |
|---|---|
| `1000` | Normal: client disconnected or server shutdown. |
| `1011` | Internal: replay or dispatch error. |
| `4000` | `slow_consumer`: dispatch dropped frames. Reconnect with `from_height` of the last height you saw. |

Auth failures during the HTTP upgrade come back as
`401 application/problem+json`, not as a close code. Two more
HTTP-side rejections are possible at upgrade time:

- **`429 stream_subject_limit`** — your JWT subject already has the
  maximum number of concurrent streams open (default 8). Reconnect
  once one of your existing connections closes.
- **`503 stream_unavailable`** — the API process's LISTEN/NOTIFY hub
  isn't running (DB connectivity blip, indexer not yet migrated).
  Same payload shape as other problem-detail responses; retry with
  backoff.

These hit at the HTTP upgrade so the client sees a normal HTTP
problem body, not a WebSocket close frame.

### Examples

```bash
# CLI (websocat or wscat)
TOKEN="$(docker compose exec api /app/jwt-issue --sub stream-cli --ttl 1h)"
wscat -c "ws://localhost:8080/api/v1/momentums/stream" \
      -H "Authorization: Bearer $TOKEN"
```

```javascript
// Browser
const url = `ws://localhost:8080/api/v1/momentums/stream?token=${encodeURIComponent(jwt)}&from_height=${lastSeen}`;
const ws = new WebSocket(url);
ws.onmessage = (e) => {
  const momentum = JSON.parse(e.data);
  console.log(momentum.height, momentum.producer_name);
};
ws.onclose = (e) => {
  if (e.code === 4000) {
    // slow_consumer — reconnect with the last height we saw
  }
};
```
