# Authentication

All endpoints under `/api/v1/` require a Bearer JWT. `/healthz`,
`/readyz`, and `/metrics` are unauthenticated.

## Scheme

| | |
|---|---|
| Type | JWT, HS256-signed |
| Header | `Authorization: Bearer <token>` |
| Claims | `sub`, `exp`, `iat`, and an OAuth 2.0 space-separated `scope` claim |
| Secret | `API_JWT_SECRET` env var on the server; never committed |

The server pins HS256 in both the JWT validation method check and via
`jwt.WithValidMethods` so an `alg=none` token cannot bypass signature
verification.

## Issuing tokens

There is no token-mint HTTP endpoint. Tokens are issued out-of-band by
an admin via the `cmd/jwt-issue` CLI bundled into the API container.

```bash
docker compose exec api /app/jwt-issue \
    --sub <consumer-name> \
    --ttl 24h \
    --scope read
```

| Flag | Default | Notes |
|---|---|---|
| `--sub` | (required) | Subject claim — identifies the caller. Used as the rate-limit key, so make it stable per-consumer (e.g. `frontend-prod`). |
| `--ttl` | `24h` | Go [`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration) format. Avoid TTLs longer than a few days — see "Rotation" below. |
| `--scope` | `read` | Comma-separated. Stored as a space-separated OAuth 2.0 scope claim. |

The `jwt-issue` binary reads `API_JWT_SECRET` from the environment;
it does **not** accept the secret on the command line.

## Scopes

The current API enforces only "valid token = read access". The
`scope` claim is preserved and exposed on `Claims.Scopes()` so that
per-route scope enforcement (`read:projects`, `read:rewards`, etc.)
can be added without changing the token format. Today, every minted
token should carry at minimum `read`.

## Rotation

HS256 uses a single shared secret — there is no key-ID field. To
rotate:

1. Generate a new secret: `openssl rand -base64 48`.
2. Restart the API with `API_JWT_SECRET` set to the new value.
3. Re-issue every active token (the old ones immediately become
   invalid; clients will see 401 until they pick up new tokens).

For zero-downtime rotation, run a second replica on the new secret
behind a load balancer and bleed traffic over as you re-issue tokens.

## Failure responses

| Condition | Status | `code` field |
|---|---|---|
| Missing `Authorization` header | 401 | `missing_token` |
| Malformed Bearer header | 401 | `invalid_token` |
| Bad signature | 401 | `invalid_token` |
| Expired token | 401 | `expired_token` |
| Wrong algorithm (e.g. `alg=none`) | 401 | `invalid_token` |
| Rate limit hit | 429 | `rate_limited` |

All failures are returned as `application/problem+json` per
[RFC 7807](https://www.rfc-editor.org/rfc/rfc7807).

### Rate limiting and unauthenticated requests

Rate limiting only applies after Auth — the middleware chain runs
`Auth → RateLimit`, so a request with no token never reaches the
rate limiter. The bucket is keyed by JWT `sub`. (`/healthz`,
`/readyz`, and `/metrics` are outside `/api/v1` entirely, so they
are unauthenticated and unrate-limited by design.)

## Local development

```bash
export API_JWT_SECRET=$(openssl rand -base64 48)
go run ./cmd/api &

TOKEN=$(API_JWT_SECRET="$API_JWT_SECRET" \
        go run ./cmd/jwt-issue --sub dev --ttl 30m)
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/status | jq
```

Pair the same `API_JWT_SECRET` on both processes — that's the entire
trust boundary.
