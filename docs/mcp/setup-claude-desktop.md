# Setup — Claude Desktop

Configure Claude Desktop to query the hosted nom-indexer MCP server.
The user never installs an MCP binary; they paste a JWT into the
config file and restart Claude Desktop.

## Prerequisites

1. The operator has the stack running and the MCP service reachable on
   a URL you can hit (`http://localhost:8081/mcp` for local dev,
   `https://mcp.example.com/mcp` for a deployed instance).
2. The operator has minted you a JWT — see [Issuing a token](#issuing-a-token).

## Issuing a token

The operator runs this once per user. The token's `sub` claim shows up
in metrics and rate limits, so make it stable per user.

```bash
docker compose exec -T mcp /app/jwt-issue \
    --sub alice \
    --ttl 720h \
    --scope read
# → eyJhbGciOiJIUzI1NiIs…
```

Hand the token to the user out-of-band (1Password, signed message,
whatever). There is no token endpoint — there is no way to mint a
token from inside Claude Desktop.

## Configure Claude Desktop

Open `claude_desktop_config.json`:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

Add an entry under `mcpServers`. If the file is empty, the full
contents should be:

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

For local dev against a `docker compose up` stack, use
`http://localhost:8081/mcp` and the locally-minted token. Restart
Claude Desktop after editing the file.

## Verify

In a fresh Claude Desktop conversation, ask:

> What's the latest momentum height on Zenon?

Claude should call `get_status` automatically. If the call fails with
an authentication error, double-check:

- the `Bearer ` prefix is present (one space, not a colon),
- the token has not expired (`exp` claim — re-issue if needed), and
- the server URL is reachable (open it in a browser — you should get
  a 405 or 400, not a connection error).

## Rotating tokens

Tokens are short-lived by design. To rotate:

1. Operator runs `jwt-issue` again with a new TTL.
2. User pastes the new token into `claude_desktop_config.json`.
3. User restarts Claude Desktop.

No server-side state changes — the old token simply expires.

## Revoking access

Because tokens are stateless (HS256 with a single shared secret), the
only way to revoke a single user mid-TTL is to rotate the server-side
secret and re-issue every active token. For per-user revocation, set a
short TTL (24h–48h) and stop re-issuing.
