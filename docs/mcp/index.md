# MCP server

The nom-indexer-go MCP server is a [Model Context
Protocol](https://modelcontextprotocol.io) endpoint that exposes the
same Postgres tables the [REST API](../api/index.md) serves, but speaks
JSON-RPC over Streamable HTTP so AI agents (Claude Desktop, Claude
Code, any tool-using LLM) can query them directly. It is hosted in
the same Docker stack as the indexer and API — **end users never run an
MCP binary locally**; they configure their client to point at the
hosted endpoint with a JWT issued by the operator.

## Quick start

```bash
# 1. Start the full stack. The indexer container runs migrations on
#    startup; until those finish, /readyz returns 503 schema_not_migrated.
docker compose up -d
# (Requires POSTGRES_PASSWORD and API_JWT_SECRET in .env.)

# 2. Mint a token. The MCP and API services share API_JWT_SECRET by
#    default (set MCP_JWT_SECRET to isolate them). The jwt-issue binary
#    reads API_JWT_SECRET unless --secret-env is provided.
export TOKEN=$(docker compose exec -T api /app/jwt-issue \
    --sub claude-desktop --ttl 24h --scope read | tail -1)

# 3. Smoke the wire.
curl -s -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize",
          "params":{"protocolVersion":"2025-06-18",
                    "capabilities":{},
                    "clientInfo":{"name":"smoke","version":"0"}}}'
```

`/healthz` and `/readyz` do not require a token; `/mcp` does. `/readyz`
returns 503 until the database is reachable **and** the indexer schema
has been applied — safe as a Kubernetes readiness probe even on a cold
cluster.

For configuring real clients, see:

- [Claude Desktop setup](setup-claude-desktop.md)
- [Claude Code setup](setup-claude-code.md)

## Architecture

The MCP server runs alongside the REST API as a sibling Docker service.
Both processes:

- read from the same Postgres pool the indexer fills,
- import the same `internal/repository` + `internal/api/dto` packages
  (so wire shapes are identical across the two transports), and
- accept the same HS256 JWT (shared `API_JWT_SECRET` unless
  `MCP_JWT_SECRET` is set).

```text
Claude Desktop ────┐
Claude Code ───────┼──► cmd/mcp :8081 /mcp ──► internal/mcp/tools ──┐
Browser MCP ───────┘                                                ├─► internal/repository ──► Postgres
                                                                    │
Frontend ──────────► cmd/api :8080 /api/v1/* ──► internal/api ──────┘
```

| | REST API | MCP server |
|---|---|---|
| Port | 8080 | 8081 |
| Metrics | 9090 | 9091 |
| Transport | HTTP + JSON | MCP over Streamable HTTP (JSON-RPC, optional SSE) |
| Auth | HS256 JWT bearer | HS256 JWT bearer (same secret by default) |
| Discovery | Swagger UI | MCP `tools/list` (auto-surfaces in clients) |
| Streaming | One WS endpoint per stream | Tool calls only in v1 — server-initiated notifications deferred |

## Transport

The MCP server speaks the **Streamable HTTP** transport at `POST /mcp`:
a single JSON-RPC endpoint that returns either a normal JSON response
or, for long-running calls, an SSE stream. This is the transport
Claude Desktop's "remote MCP server" config expects, and it replaces
the older SSE-only transport in the spec.

There is no stdio mode — by design. End users never run an MCP binary
locally; the server is hosted.

## Authentication

Every call to `/mcp` must include an HS256 JWT, either:

- `Authorization: Bearer <token>` (CLI / server-side clients), or
- `?token=<token>` query parameter (browser MCP clients that cannot
  set custom headers on the upgrade request).

The signer is shared with the REST API by default. Operators mint tokens
with the bundled `cmd/jwt-issue` CLI; there is no token endpoint.

```bash
# Mint a 30-day token for a single Claude Desktop user.
docker compose exec -T mcp /app/jwt-issue \
    --sub alice --ttl 720h --scope read
```

If `MCP_JWT_SECRET` is set, point the issuer at that environment
variable so the token is signed with the MCP-only secret:

```bash
docker compose exec -T mcp /app/jwt-issue \
    --secret-env MCP_JWT_SECRET \
    --sub alice --ttl 720h --scope read
```

On failure the server returns HTTP 401 with a JSON-RPC error envelope:

```json
{
  "jsonrpc": "2.0",
  "id": null,
  "error": {
    "code": -32001,
    "message": "invalid or expired token",
    "data": { "code": "expired_token" }
  }
}
```

`data.code` is one of: `missing_token`, `invalid_token`, `expired_token`.

## Tool catalog

Tools are one-per-logical-query and mirror the REST endpoints — see
[Tools](tools.md) for the full list. As of v1 there are 33 tools across
the same domains the REST API surfaces (momentums, accounts, tokens,
pillars, sentinels, stakes, fusions, projects, rewards, bridge).

Each tool returns the same `dto.*` shape the REST API emits: amounts as
JSON strings, paginated envelopes for lists, etc. Moving between
transports doesn't require re-learning the data.

## Resources

v1 advertises **no MCP resources**. The schema catalog formerly served
at `schema://overview` is now the `get_schema_overview` tool — same
payload, called by the LLM on demand rather than surfaced as a
manual-attach affordance in the client UI. See
[Resources](resources.md) for the rationale and
[Tools](tools.md#status--schema) for the tool entry.

## Observability

- `/healthz` — liveness, always 200.
- `/readyz` — DB ping + `schema_migrations.version >= 12`.
- `/metrics` (on `:9091`, separate listener) — Prometheus exposition
  with `nom_mcp_tool_calls_total{tool,status}` and
  `nom_mcp_tool_call_duration_seconds{tool,status}` plus the standard
  Go/process collectors.

## Limits and follow-ups

v1 is intentionally minimal. See
[Known issues](../reference/known-issues.md) for the documented
deferrals — most notably:

- No server-initiated notifications (the LLM can't subscribe to a live
  momentum stream). The REST API's WebSocket endpoints fill this gap
  for non-MCP consumers; MCP-side subscription semantics are a future
  design.
- No per-tool scope enforcement — every authenticated caller can call
  every tool. The `scope` claim is preserved for future per-route
  gating.
- No cursor pagination — mirrors the REST-side gap.
