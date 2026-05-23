# REST API

The nom-indexer-go HTTP API is a read-only layer over the indexed
Postgres database. The OpenAPI 3.1 contract at
[`openapi.yaml`](openapi.yaml) is the source of truth; this section
documents the conventions that wrap it.

## Quick start

```bash
# 1. Start the API + Postgres (requires .env with API_JWT_SECRET set)
docker compose up -d api

# 2. Mint a token (admin runs this — there is no token endpoint)
docker compose exec api /app/jwt-issue \
    --sub frontend-dev --ttl 24h --scope read

# 3. Call any endpoint
TOKEN="paste the token here"
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/status | jq

curl -s -H "Authorization: Bearer $TOKEN" \
     'http://localhost:8080/api/v1/momentums?page=1&page_size=10' | jq
```

`/healthz` and `/readyz` do not require a token; everything under
`/api/v1/` does.

## Interactive Swagger UI

<swagger-ui src="openapi.yaml"/>

## Conventions

### Response shape

* **Collection endpoints** return an envelope:

  ```json
  {
    "data": [...],
    "pagination": { "page": 1, "page_size": 50, "total": 12345 }
  }
  ```

* **Single-object endpoints** return the object directly (no envelope).
* **Errors** are RFC 7807 `application/problem+json`:

  ```json
  { "type": "about:blank", "title": "Unauthorized",
    "status": 401, "detail": "missing bearer token",
    "code": "unauthorized" }
  ```

### Amounts are JSON strings

Token amounts (`balance`, `total_supply`, `znn_amount`, etc.) ship as
**JSON strings** rather than numbers. The raw int64 values stored by
the indexer regularly exceed `Number.MAX_SAFE_INTEGER` (`2^53 - 1`),
so a numeric JSON field would silently lose precision in JavaScript
clients. Convert with `BigInt(value)` (JS) or `int.Parse(value)` (Go).

### Pagination

See [Pagination](pagination.md) for the offset/limit contract.

### Authentication

See [Authentication](auth.md) for token issuance, headers, and rotation.

### Rate limit

In-process token-bucket: **60 requests/minute** per JWT subject
(falling back to IP if no token). Returns `429` with a
problem+json body when exceeded. Limit is per-replica — multi-replica
deployments multiply the effective ceiling.

### Versioning

All current endpoints live under `/api/v1/`. A future v2 will be added
as a sibling subtree; v1 will continue to receive bug fixes.

## Where to look next

| You want… | Page |
|---|---|
| The full endpoint catalog | [Endpoints overview](endpoints/index.md) |
| To issue or rotate tokens | [Authentication](auth.md) |
| The pagination + sort contract | [Pagination](pagination.md) |
| Schema-level column meanings | [Schema reference](../schema/index.md) |
| What's deferred to a future release | [Known issues](../reference/known-issues.md) |

## LLM-friendly access

This site exposes [`/llms.txt`](https://0x3639.github.io/nom-indexer-go/llms.txt)
and [`/llms-full.txt`](https://0x3639.github.io/nom-indexer-go/llms-full.txt)
following the [llms.txt convention](https://llmstxt.org). The API
contract itself is at [`/api/openapi.yaml`](openapi.yaml) — point any
OpenAPI-aware tool at it.
