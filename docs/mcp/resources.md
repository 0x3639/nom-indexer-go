# MCP resources

MCP resources are read-only context the LLM can pull before reaching
for [tools](tools.md) — useful for grounding tool selection in the
underlying data model. v1 ships one resource.

## `schema://overview`

Compact JSON catalog of every Postgres table the indexer fills, with
the MCP tools that read each one. Pull this first when the user's
question doesn't obviously map to one tool.

| | |
|---|---|
| URI | `schema://overview` |
| MIME type | `application/json` |
| Size | ~3 KB |
| Backed by | Hand-curated literal in `internal/mcp/resources/schema.go` |

### Wire shape

```json
{
  "version": 1,
  "tables": [
    {
      "name": "momentums",
      "domain": "core_ledger",
      "purpose": "Block headers indexed by height.",
      "tools": ["get_momentum_by_height", "get_latest_momentum", "list_momentums", "get_status"]
    }
    // ... 28 more rows
  ],
  "notes": [
    "All amounts/balances/supplies are int64 (BIGINT) — JSON-encoded as strings to dodge JavaScript Number precision loss.",
    "All timestamps are Unix seconds (BIGINT).",
    "All hashes and addresses are lowercase hex/zenon strings.",
    "No foreign keys: joins are by hash/address/standard. The indexer writes per momentum in a transaction."
  ]
}
```

### Domains

Tables are bucketed into seven domains; one row per table:

- `core_ledger` — momentums, account_blocks, accounts, balances,
  tokens, token_mints, token_burns
- `pillars` — pillars, pillar_updates, delegations
- `sentinels_stakes_plasma` — sentinels, stakes, fusions
- `accelerator_z` — projects, project_phases, votes
- `rewards` — reward_transactions, cumulative_rewards
- `bridge` — wrap_token_requests, unwrap_token_requests, bridge_*
- `daily_snapshots` — network/token/pillar/bridge stat histories

### Reading the resource

In a real MCP client (Claude Desktop, Claude Code) the resource is
auto-discovered via `resources/list` and pulled on demand. By hand:

```bash
TOKEN="paste your token"
curl -s -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":1,"method":"resources/read",
          "params":{"uri":"schema://overview"}}'
```

### Keeping it current

The catalog is hand-curated rather than auto-derived: it's
LLM-facing and benefits from concise phrasing. When the schema gains
a new table OR the tool catalog changes, update both
`internal/mcp/resources/schema.go` AND this page. A test in
`internal/mcp/resources/schema_test.go` asserts that a representative
sample of tables is present, but it does not catch a missing entry —
the linter will not nag you if you forget. The cross-link to the
canonical schema reference is [docs/schema/](../schema/index.md).

## Future resources

These have not been built; see [Known issues](../reference/known-issues.md)
for the full deferral list.

- `docs://migrations` — a flat catalog of migrations + their semantics,
  useful when the LLM is reasoning about whether a column exists yet.
- `schema://table/<name>` — per-table deep dive with column types,
  index hints, and gotchas (the same content the docs/schema/ pages
  render in MkDocs, but in MCP form).
