# Pagination

Collection endpoints use **offset/limit** pagination with a fixed
envelope. Single-object endpoints (`GET /api/v1/momentums/{height}`,
`GET /api/v1/tokens/{token_standard}`, etc.) return the object
directly with no envelope.

## Query parameters

| Param | Default | Bounds | Notes |
|---|---|---|---|
| `page` | `1` | ≥ 1 | 1-based. Out-of-range silently clamped to 1. |
| `page_size` | `50` | 1 ≤ n ≤ 200 | Out-of-range silently clamped into bounds. |
| `sort` | depends on endpoint | `asc` \| `desc` | Honored only by endpoints with a documented sort column. Invalid values fall back to the endpoint's default. |

The API never returns 400 on pagination misuse — bad values are
clamped instead. This keeps consumer code that builds query strings
defensively from breaking.

## Response envelope

```json
{
  "data": [ { ... }, { ... } ],
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total": 12345
  }
}
```

| Field | Type | Meaning |
|---|---|---|
| `data` | array | The page's rows. Always present — empty page returns `[]`, never `null`. |
| `pagination.page` | int | Echo of the requested page (after clamping). |
| `pagination.page_size` | int | Echo of the requested page_size (after clamping). |
| `pagination.total` | int64 | Total rows matching the query across all pages, not just the current page. **Exact** for scoped queries (by-address, by-pillar, by-token, etc.); **approximate** for the two whole-table list endpoints noted below. |

For most scoped endpoints `total` is computed inline via
`COUNT(*) OVER ()` — one round trip per request, exact, no
LIMIT/OFFSET drift. Address-scoped account transactions are the main
exception: `/api/v1/accounts/{address}/transactions` reads the exact
cached [`accounts.tx_count`](../schema/accounts.md) counter instead of
counting `account_blocks` at request time.

### Approximate totals on large list endpoints

`GET /api/v1/momentums` and `GET /api/v1/account_blocks` are the only
two endpoints that paginate without a filter across tables that grow
linearly with the chain. Computing an exact `COUNT(*) OVER ()` on
each request forced Postgres to scan the entire 13M-row momentums
table even on page 1, which blew past the API's 30s server timeout.

The page query now skips the window aggregate; `total` comes from a
cheap-but-approximate source per table:

| Endpoint | Source | Drift |
|---|---|---|
| `/api/v1/momentums` | `SELECT MAX(height) FROM momentums` | Upper bound — equals the row count only when there are no backfill gaps. ~15 ms. |
| `/api/v1/account_blocks` | `SELECT reltuples FROM pg_class` | Lag matches the autovacuum cadence (Postgres default: every few minutes for a hot table). ~5 ms. |

Scoped variants such as `/api/v1/pillars/{name}/delegators` keep the
exact inline count — the WHERE clause shrinks the scan to a tractable
size. `/api/v1/accounts/{address}/transactions` is also exact, but its
total comes from `accounts.tx_count` because busy addresses can still
have hundreds of thousands of matching account blocks.

## Honest tradeoff: deep pagination on large tables

Offset pagination requires Postgres to scan and discard `OFFSET`
rows before returning `LIMIT` — at page 10,000 of 50 that's 500,000
rows skipped per request. Performance degrades linearly with
offset, even on the new fast-count list endpoints (the count is
fast, the offset isn't).

**For now:** clients fetching deep history should narrow with
filters (e.g. `?from_height=...&to_height=...` once those filters
land) rather than paging blindly to the end.

**Future:** the plan reserves cursor pagination for v2. When that
ships, the envelope will gain a `next_cursor` field while keeping the
existing offset/limit shape for compatibility.

## Sort

Endpoints with a `sort` parameter document the column they sort by.
Typical defaults:

| Endpoint | Default sort |
|---|---|
| `/api/v1/momentums` | `desc` by `height` |
| `/api/v1/account_blocks` | `desc` by `momentum_height` |
| `/api/v1/accounts/{address}/transactions` | `desc` by `momentum_height` |

Endpoints with implicit ordering (e.g. `/api/v1/tokens` is always
sorted by `holder_count DESC`) intentionally do not accept `sort`.

## Total count cost

For most filtered queries against indexed columns, `COUNT(*) OVER ()`
is one round trip and the planner can satisfy it from the same scan
that produces the page. For the account transactions endpoint, even
the filtered scan can be too large for busy addresses, so it uses the
cached exact `accounts.tx_count` counter documented in
[Accounts](endpoints/accounts.md). For the two large-table list
endpoints noted above, the cost was prohibitive at production scale,
so they switched to the approximate sources documented in
[Approximate totals on large list endpoints](#approximate-totals-on-large-list-endpoints).
