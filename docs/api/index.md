# API (forthcoming)

A REST API for the indexer is planned. This page is a stub; endpoint references
will live under `docs/api/endpoints/` once the server ships.

## Status

| Item | State |
|---|---|
| Server implementation | Not started |
| Draft implementation brief | [`specs/API_SPECIFICATION.md`](https://github.com/0x3639/nom-indexer-go/blob/main/specs/API_SPECIFICATION.md) (973-line spec) |
| OpenAPI 3.1 contract | [`openapi.yaml`](openapi.yaml) (placeholder skeleton) |
| Rendered Swagger UI | Will load automatically here when `openapi.yaml` is filled in. |

## What this API will surface

The API is a **read-only HTTP layer over the database tables this indexer
fills**. There is no application logic on top — the schema is the contract.

If you're designing a query, start in the [schema reference](../schema/index.md)
and identify the relevant table(s). The API will map endpoints onto those
tables.

## Where the contract lives

- **Tables and columns:** [`docs/schema/`](../schema/index.md)
- **Schema-wide conventions** (int64 cap, timestamps, hash encoding):
  [`docs/schema/conventions.md`](../schema/conventions.md)
- **Implementation brief (draft):**
  [`specs/API_SPECIFICATION.md`](https://github.com/0x3639/nom-indexer-go/blob/main/specs/API_SPECIFICATION.md)
- **Glossary of Zenon terms:** [`docs/reference/glossary.md`](../reference/glossary.md)

## Future structure of this section

```
docs/api/
  index.md               # this file — overview + status
  openapi.yaml           # the OpenAPI 3.1 contract (rendered inline as Swagger UI)
  auth.md                # authentication scheme
  pagination.md          # pagination conventions
  endpoints/
    <one-file-per-endpoint-or-group>.md
```

The directory layout is in place now so that filling in the API docs is purely
additive: no nav reshuffle, no cross-link breakage.
