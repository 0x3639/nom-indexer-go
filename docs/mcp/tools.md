# MCP tools

Every tool the [MCP server](index.md) exposes, grouped by domain. Names
and shapes match the equivalent [REST endpoints](../api/index.md) — the
two surfaces share `internal/api/dto` and `internal/repository`, so a
caller can move between transports without re-learning the data.

## Conventions

- **Pagination params:** `page` (1-indexed, default 1) and `page_size`
  (default 50, max 200) — same caps as the REST API.
- **Sort param:** `sort` is `asc` or `desc`, defaults to the per-tool
  natural order (height DESC, timestamp DESC).
- **Output:** every tool returns both:
  - a TextContent block holding pretty-printed JSON (for LLM
    consumption), and
  - `structuredContent` carrying the same payload as a typed object
    (for programmatic consumers).
- **Envelopes:**
  - List tools return `{"data": [...], "pagination": {...}}`.
  - Unpaginated lists (account balances, project phases, cumulative
    rewards) return `{"data": [...]}` with no pagination block.
  - Single-object tools return the object directly.
- **Amounts:** every `*_amount`, `total_supply`, `max_supply`, etc.
  ships as a JSON string. The underlying int64 values regularly exceed
  2^53, so a JS Number would lose precision.

## Status + schema

| Tool | Input | Output |
|---|---|---|
| `get_status` | — | `dto.Status` — `{latest_height, latest_timestamp, indexer_lag_seconds, version}` |
| `get_schema_overview` | — | `{version, tables: [{name, domain, purpose, tools}], notes: [string]}` — compact catalog of every indexed table with the tools that read it. Call first to ground tool selection when the question doesn't map to one tool. |

## Momentums (block headers)

| Tool | Input | Output |
|---|---|---|
| `get_momentum_by_height` | `height: int64` | `dto.Momentum` |
| `get_latest_momentum` | — | `dto.Momentum` |
| `list_momentums` | `page, page_size, sort` | `Page<Momentum>` |

## Accounts and balances

| Tool | Input | Output |
|---|---|---|
| `get_account` | `address: string` | `dto.Account` |
| `list_account_balances` | `address` | `{data: [Balance]}` (unpaginated; balances per account are bounded) |
| `list_account_transactions` | `address, page, page_size, sort` | `Page<AccountBlock>` |

## Account blocks (transactions)

| Tool | Input | Output |
|---|---|---|
| `list_account_blocks` | `page, page_size, sort` | `Page<AccountBlock>` |
| `get_account_block` | `hash` | `dto.AccountBlock` |

## Tokens

| Tool | Input | Output |
|---|---|---|
| `list_tokens` | `page, page_size` | `Page<Token>` |
| `get_token` | `token_standard` (zts1…) | `dto.Token` |
| `list_token_holders` | `token_standard, page, page_size` | `Page<Balance>` — richlist for one token |

## Pillars

| Tool | Input | Output |
|---|---|---|
| `list_pillars` | `include_revoked, page, page_size` | `Page<Pillar>` |
| `get_pillar_by_name` | `name` | `dto.Pillar` |
| `list_pillar_delegators` | `name, page, page_size` | `Page<PillarDelegator>` — resolves the pillar name to its owner address first |

## Sentinels

| Tool | Input | Output |
|---|---|---|
| `list_sentinels` | `include_inactive, page, page_size` | `Page<Sentinel>` |

## Stakes (ZNN delegation entries)

| Tool | Input | Output |
|---|---|---|
| `list_stakes` | `include_inactive, page, page_size` | `Page<Stake>` — active only by default |
| `list_account_stakes` | `address, include_inactive, page, page_size` | `Page<Stake>` |

## Fusions (QSR plasma entries)

| Tool | Input | Output |
|---|---|---|
| `list_fusions` | `include_inactive, page, page_size` | `Page<Fusion>` |
| `list_account_fusions` | `address, include_inactive, page, page_size` | `Page<Fusion>` — matches on funder OR beneficiary |

## Rewards

| Tool | Input | Output |
|---|---|---|
| `list_account_rewards` | `address, page, page_size` | `Page<RewardTransaction>` — per-event history |
| `get_account_cumulative_rewards` | `address` | `{data: [CumulativeReward]}` — one row per (reward_type, token_standard) |

## Accelerator-Z (projects)

| Tool | Input | Output |
|---|---|---|
| `list_projects` | `page, page_size` | `Page<Project>` |
| `get_project` | `id` (64-char hex) | `dto.Project` |
| `list_project_phases` | `id` | `{data: [ProjectPhase]}` — ascending phase order |
| `list_project_votes` | `id, page, page_size` | `Page<Vote>` — pillar votes on the project or any of its phases |

## Bridge

| Tool | Input | Output |
|---|---|---|
| `list_bridge_wraps` | `page, page_size` | `Page<WrapTokenRequest>` — ZTS → external chain intents |
| `list_bridge_unwraps` | `page, page_size` | `Page<UnwrapTokenRequest>` |
| `list_account_bridge_wraps` | `address, page, page_size` | `Page<WrapTokenRequest>` |
| `list_account_bridge_unwraps` | `address, page, page_size` | `Page<UnwrapTokenRequest>` |

## Calling a tool by hand

The Streamable HTTP transport accepts plain JSON-RPC, so any HTTP
client works. The example below smoke-tests `get_status` end-to-end.

```bash
TOKEN="paste your token"
HEADERS="$(mktemp)"

# 1. initialize — required handshake. Capture the session id header.
curl -sS -D "$HEADERS" -o /tmp/nom-mcp-init.out \
     -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize",
          "params":{"protocolVersion":"2025-06-18",
                    "capabilities":{},
                    "clientInfo":{"name":"smoke","version":"0"}}}'
SESSION="$(awk 'tolower($1)=="mcp-session-id:" {gsub("\r","",$2); print $2}' "$HEADERS")"

# 2. initialized — completes the lifecycle notification.
curl -sS -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Mcp-Session-Id: $SESSION" \
     -H "MCP-Protocol-Version: 2025-06-18" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 3. tools/list — discover the catalog.
curl -sS -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Mcp-Session-Id: $SESSION" \
     -H "MCP-Protocol-Version: 2025-06-18" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# 4. tools/call get_status.
curl -sS -X POST http://localhost:8081/mcp \
     -H "Authorization: Bearer $TOKEN" \
     -H "Mcp-Session-Id: $SESSION" \
     -H "MCP-Protocol-Version: 2025-06-18" \
     -H "Content-Type: application/json" \
     -H "Accept: application/json, text/event-stream" \
     -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
          "params":{"name":"get_status","arguments":{}}}'
```

In real clients (Claude Desktop, Claude Code) the handshake and
discovery are handled automatically — see the
[Claude Desktop setup](setup-claude-desktop.md) and
[Claude Code setup](setup-claude-code.md) pages.
