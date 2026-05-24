# Setup — Claude Code

Configure [Claude Code](https://claude.com/claude-code) to query the
hosted nom-indexer MCP server. Same shape as the
[Claude Desktop](setup-claude-desktop.md) flow — the only difference is
where the config lives.

## Prerequisites

1. The operator has the stack running and the MCP service reachable on
   a URL you can hit.
2. The operator has minted you a JWT (see the Claude Desktop guide for
   the `jwt-issue` invocation, including the `--secret-env
   MCP_JWT_SECRET` flag when MCP uses isolated key material).

## Add the MCP server

From the terminal, with Claude Code installed:

```bash
claude mcp add nom-indexer \
    --transport http \
    --url https://mcp.example.com/mcp \
    --header "Authorization: Bearer eyJhbGciOiJIUzI1NiIs…"
```

This writes the entry to the project-scoped `.mcp.json` (or
user-scoped, depending on Claude Code's current settings) so the
server is available in every session in this project.

Equivalent `.mcp.json` snippet for hand-editing:

```json
{
  "mcpServers": {
    "nom-indexer": {
      "type": "http",
      "url": "https://mcp.example.com/mcp",
      "headers": {
        "Authorization": "Bearer eyJhbGciOiJIUzI1NiIs…"
      }
    }
  }
}
```

For local dev: `--url http://localhost:8081/mcp` and the locally-minted
token.

## Verify

```text
> What's the latest momentum height on Zenon?
```

Claude Code calls `get_status` and reports the height. If you don't
see a tool-call indicator in the response, the server probably failed
to register — check `claude mcp list` and `claude mcp logs nom-indexer`.

## Token rotation

Same as Claude Desktop. Re-run `claude mcp add` with `--force` or
edit `.mcp.json` directly with the new token.
