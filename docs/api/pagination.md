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
| `pagination.total` | int64 | Total rows matching the query across all pages, not just the current page. |

`total` is computed in the same SQL statement via `COUNT(*) OVER ()`
— one round trip per request, but the count is exact (no
LIMIT/OFFSET drift).

## Honest tradeoff: deep pagination on large tables

`account_blocks` is the largest table the indexer fills (tens of
millions of rows). Offset pagination requires Postgres to scan and
discard `OFFSET` rows before returning `LIMIT` — at page 10,000 of
50 that's 500,000 rows skipped per request. Performance degrades
linearly with offset.

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

`COUNT(*) OVER ()` is one query, but the planner still scans the
matching set to compute it. On filtered queries against indexed
columns this is fast; on unfiltered scans of the largest tables it
matches the cost of the `LIMIT` query. If you only need the next
page and don't care about the total, that's still cheap — the count
is computed once and returned alongside the rows.
