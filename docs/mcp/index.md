# MCP server (forthcoming)

A Model Context Protocol server is planned. This page is a stub; tool and
resource references will live under `docs/mcp/tools.md` and
`docs/mcp/resources.md` once the server ships.

## Status

| Item | State |
|---|---|
| Server implementation | Not started |
| Draft tool / resource list | Stub pages only; no runtime contract yet |

## What this MCP server will expose

The MCP server exposes the indexer's database to MCP-capable clients
(Claude, Cursor, IDEs). It is **read-only** and built directly on the schema
this indexer fills.

- **Resources** — each table becomes one or more MCP resources for read-only
  retrieval. Defined in [`docs/mcp/resources.md`](resources.md).
- **Tools** — parameterized queries that answer common questions ("rewards
  by address", "pillar by name", "wrap requests for a token"). Defined in
  [`docs/mcp/tools.md`](tools.md).

## Where the contract lives

- **Tables and columns:** [`docs/schema/`](../schema/index.md)
- **Schema-wide conventions:** [`docs/schema/conventions.md`](../schema/conventions.md)
- **Glossary of Zenon terms:** [`docs/reference/glossary.md`](../reference/glossary.md)

## Future structure of this section

```
docs/mcp/
  index.md          # this file — overview + status
  resources.md      # resource list keyed by table
  tools.md          # tool list with input schemas and example invocations
```

The directory layout is in place now so that filling in the MCP docs is purely
additive.
