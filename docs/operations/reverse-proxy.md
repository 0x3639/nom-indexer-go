---
title: Reverse proxy + TLS (public exposure)
---

# Reverse proxy + TLS (public exposure)

The indexer itself has no public surface — only the **API** (`cmd/api`,
default `:8080`) and **MCP** (`cmd/mcp`, default `:8081`) services are meant
to face the internet, and both already require a Bearer JWT on every
`/api/v1/*` and `/mcp` request. To expose them safely you need three things:
TLS termination, the private ports kept private, and a host firewall.

This page documents the bundled **Caddy** overlay, which gives you automatic
Let's Encrypt certificates with almost no config.

The repo ships two ready files — neither needs editing; both are driven by
environment variables in your `.env`:

- [`configs/Caddyfile`](https://github.com/0x3639/nom-indexer-go/blob/main/configs/Caddyfile)
  — the proxy config. Domains and ACME email are `{$API_DOMAIN}`,
  `{$MCP_DOMAIN}`, `{$ACME_EMAIL}` placeholders, so the same file works in
  every environment.
- [`docker-compose.caddy.yml`](https://github.com/0x3639/nom-indexer-go/blob/main/docker-compose.caddy.yml)
  — an additive overlay adding only the `caddy` service. It passes those env
  vars to the container and defaults them to `*.localhost` for local use.

## Local vs production

Caddy is **opt-in** — it's a separate compose overlay, so you choose per
environment whether to include it:

| | Local dev | Production |
|---|---|---|
| Compose | base only | base **+** `-f docker-compose.caddy.yml` |
| Reach API/MCP | direct: `http://localhost:8080` / `:8081` | `https://api.example.com` via Caddy |
| TLS | none | Caddy auto Let's Encrypt |
| `.env` domains | unset → defaults to `*.localhost` | real public subdomains |

**The simplest local workflow is to skip Caddy entirely** and hit the host
ports directly — which is how the rest of these docs' examples work. Only
include the overlay when you're on the public server, or when you
specifically want to exercise the proxy path locally (see
[Local testing](#local-testing)).

## What is (and isn't) exposed

| Service / port | Exposure |
|---|---|
| Caddy `80`, `443` (+`443/udp`) | **Public** — the only internet-facing ports. |
| API `8080`, MCP `8081` | **Private.** Caddy reaches them over the internal `nom-indexer` docker network by service name. The overlay rebinds their host publishes to loopback. |
| API metrics `9090`, MCP metrics `9091` | **Private — never proxy these.** `/metrics` is unauthenticated. They live on separate ports, so fronting the API/MCP ports never exposes them. |
| Postgres `5432` | **Private.** The overlay rebinds it to loopback; never expose it. |
| znnd P2P `35995/tcp+udp` | Public (peer connectivity). RPC/WS stay internal. |

## Why loopback binding matters (the overlay does it for you)

Docker programs its own `iptables` rules that are evaluated **before**
`ufw`, so `ufw deny 8080` does **not** block a Docker-published port on a
bare VPS — a firewall alone is not enough. So the overlay
(`docker-compose.caddy.yml`) rebinds Postgres, the API, MCP, and both
metrics ports to `127.0.0.1`, leaving only Caddy's `80`/`443` public. It
does this with Compose's `!override` tag (requires **Compose v2.24+**), which
replaces the base `ports` list rather than appending to it — equivalent to:

```yaml
  api:
    ports: !override
      - "127.0.0.1:${API_PORT:-8080}:${API_PORT:-8080}"
      - "127.0.0.1:${API_METRICS_PORT:-9090}:${API_METRICS_PORT:-9090}"
```

You don't apply this by hand — including the overlay is what makes the
documented production command secure. Verify the rendered result with:

```bash
docker compose -f docker-compose.yml -f docker-compose.caddy.yml config | grep -E 'published|host_ip'
# every app/metrics/postgres entry should show host_ip: 127.0.0.1
```

Add a **cloud security group** too if your provider has one — it filters at
the network edge (before Docker's rules) and is good defence in depth.

> **Fronting the services some other way** (external load balancer, a Caddy
> on a separate host)? You're not using this overlay, so apply the loopback
> binding yourself — copy the `!override` block above into your own override,
> or rely on a security group.

### Upstream ports track `API_PORT` / `MCP_PORT`

The overlay passes `API_PORT` / `MCP_PORT` into the Caddy container, and the
Caddyfile proxies to `api:{$API_PORT}` / `mcp:{$MCP_PORT}`. So if you change
either port in `.env`, the listener, the host publish, and Caddy's upstream
all stay in sync — no Caddyfile edit needed.

## Production steps

1. **Set the domains + email in `.env`** (no file edits needed):

   ```bash
   API_DOMAIN=api.example.com
   MCP_DOMAIN=mcp.example.com
   ACME_EMAIL=you@example.com
   ```

   The `Caddyfile` reads these as `{$API_DOMAIN}` / `{$MCP_DOMAIN}` /
   `{$ACME_EMAIL}`. (Its MCP block intentionally omits `encode` and sets
   `flush_interval -1` so MCP's Streamable HTTP / SSE transport streams
   instead of buffering.)

2. **DNS** — point both subdomains at the server *before* starting Caddy
   (ACME validates over ports 80/443):

   ```
   api.example.com   A    <server-ipv4>     (+ AAAA if IPv6)
   mcp.example.com   A    <server-ipv4>
   ```

3. **CORS** (only if browsers call the services directly) — in `.env`:

   ```bash
   API_CORS_ALLOWED_ORIGINS=https://yourapp.example.com
   MCP_CORS_ALLOWED_ORIGINS=https://yourapp.example.com
   ```

4. **Firewall / security group** — allow inbound only `80`, `443`, `22`
   (restricted to your IP), and `35995 tcp+udp`. Deny the rest. Ensure no
   other process holds 80/443.

5. **Bring it up** alongside the base file:

   ```bash
   docker compose -f docker-compose.yml -f docker-compose.caddy.yml --profile local-node up -d
   docker logs nom-indexer-caddy -f   # watch cert issuance
   ```

   Auto-load variant: `mv docker-compose.caddy.yml docker-compose.override.yml`
   and Compose includes Caddy on every `docker compose ... up -d`.

6. **Verify from off-box:**

   ```bash
   curl -s https://api.example.com/healthz      # {"status":"ok"}
   curl -s https://api.example.com/readyz        # {"status":"ready"}
   TOKEN=$(docker compose exec api /app/jwt-issue --sub prod --ttl 24h --scope read)
   curl -s -H "Authorization: Bearer $TOKEN" https://api.example.com/api/v1/status
   curl -s https://mcp.example.com/healthz       # {"status":"ok"}
   ```

   Confirm the private ports are unreachable from the internet:
   `curl http://<public-ip>:8080/...` and `:9090/metrics` should both fail.

## Local testing

You usually **don't need Caddy locally** — omit the overlay and use the
host ports directly (`curl http://localhost:8080/healthz`). Reach for the
proxy locally only to test the edge itself: TLS, the SSE/streaming path, or
the forwarded headers.

When you do, leave `API_DOMAIN` / `MCP_DOMAIN` / `ACME_EMAIL` **unset**. The
overlay defaults them to `api.localhost` / `mcp.localhost` / `admin@localhost`,
and Caddy serves those names with its **internal self-signed CA** — no public
DNS, no Let's Encrypt, no email.

```bash
# Linux resolves *.localhost to loopback automatically.
# macOS does NOT — add the hostnames first:
echo "127.0.0.1 api.localhost mcp.localhost" | sudo tee -a /etc/hosts

# Bring it up (drop --profile local-node if you point at an external node):
docker compose -f docker-compose.yml -f docker-compose.caddy.yml up -d

# -k accepts Caddy's internal CA (or trust it: the root is in the caddy_data
# volume at /data/caddy/pki/authorities/local/root.crt).
curl -k https://api.localhost/healthz     # {"status":"ok"}
curl -k https://mcp.localhost/healthz      # {"status":"ok"}
```

Two local gotchas: Caddy still binds host ports **80/443**, so free them
first; and a browser hitting `https://api.localhost` will record the HSTS
header for that name (harmless, but it pins HTTPS for `*.localhost`).

## Notes

- **Cert persistence.** Caddy stores ACME certs/keys in the `caddy_data`
  volume. Don't delete it, or you risk Let's Encrypt rate limits. While
  iterating, use the staging `acme_ca` line (commented in the Caddyfile).
- **Auth + rate limiting** are enforced app-side: every `/api/v1/*` and
  `/mcp` call needs a Bearer JWT, with a per-`sub` limit
  (`API_RATE_LIMIT_PER_MINUTE` / `MCP_RATE_LIMIT_PER_MINUTE`). See
  [`api/auth.md`](../api/auth.md). Tokens are minted out-of-band with
  `cmd/jwt-issue`; rotation = re-issue.
- **Pre-auth IP throttling.** Stock Caddy has no rate-limit module. To
  throttle unauthenticated abuse at the edge, front it with a CDN/WAF
  (e.g. Cloudflare) or build a custom Caddy image with the
  [`caddy-ratelimit`](https://github.com/mholt/caddy-ratelimit) plugin.
- **Single domain.** Two subdomains is cleanest. A single host with path
  routing (`/api/*` → api, `/mcp` → mcp) works too but needs path rewrites.

## See also

- [`deploy.md`](deploy.md) — the base deployment.
- [`config/node-selection.md`](../config/node-selection.md) — node tradeoffs.
- [`api/auth.md`](../api/auth.md) — the JWT scheme behind the proxy.
