---
title: Webhooks
---

# Webhooks

The indexer can push event notifications to external HTTP endpoints as it
ingests the chain. This is an opt-in, **indexer-process-only** subsystem
(`cmd/indexer`); the API and MCP processes ignore it.

Events fire **after** each momentum's database transaction commits, so a
subscriber only ever sees data that is already durable in Postgres. The
dispatcher is fire-and-forget: it never blocks the sync loop, and failed
deliveries are retried a bounded number of times and then dropped.

## Enabling webhooks

Webhooks are disabled by default. Turn the subsystem on with
`webhooks.enabled` (env `WEBHOOKS_ENABLED=true`), then list one or more
endpoints. **Endpoints are YAML-only** — there is no env var for the
endpoint list, secrets, or per-endpoint event filters.

```yaml
webhooks:
  enabled: true
  timeout_seconds: 5      # per-request HTTP timeout
  max_retries: 3          # resend attempts on failure
  endpoints:
    - url: "https://example.com/hook"
      secret: "change-me"            # signs X-Webhook-Signature (HMAC-SHA256)
      events: ["momentum.inserted", "account_block.inserted"]
    - url: "https://ops.internal/momentums"
      # no secret -> requests are unsigned
      # events omitted -> this endpoint receives ALL event types
```

Per-endpoint fields:

| Field | Required | Description |
|---|---|---|
| `url` | yes | Destination URL. Each event is delivered as an HTTP `POST` with a JSON body. |
| `secret` | no | If set, requests carry an `X-Webhook-Signature` HMAC. If empty/omitted, requests are unsigned. |
| `events` | no | Allowlist of event types this endpoint receives. **Empty or omitted = all events.** |

See [`config/reference.md`](../config/reference.md#webhooks-cmdindexer-only)
for the full key/env/default table.

## Event types

Every delivery is a JSON object with this envelope:

```json
{
  "type": "<event-type>",
  "payload": { /* event-specific fields */ }
}
```

### `momentum.inserted`

Fires once per momentum, after the momentum (and all its account blocks)
are committed.

```json
{
  "type": "momentum.inserted",
  "payload": {
    "height": 1234567,
    "hash": "0a1b2c…",
    "timestamp": 1733500800
  }
}
```

| Field | Type | Description |
|---|---|---|
| `height` | number | Momentum height. |
| `hash` | string | Momentum hash, 64-char lowercase hex. |
| `timestamp` | number | Momentum time, Unix seconds. |

### `account_block.inserted`

Fires once per account block contained in the committed momentum. A single
momentum can produce many of these events. They are emitted after the
`momentum.inserted` event for the same momentum.

```json
{
  "type": "account_block.inserted",
  "payload": {
    "momentumHeight": 1234567,
    "hash": "0a1b2c…",
    "address": "z1q…",
    "toAddress": "z1q…",
    "blockType": 2
  }
}
```

| Field | Type | Description |
|---|---|---|
| `momentumHeight` | number | Height of the momentum that confirmed this block. |
| `hash` | string | Account block hash, 64-char lowercase hex. |
| `address` | string | Sender address (Bech32). |
| `toAddress` | string | Recipient address (Bech32). |
| `blockType` | number | Numeric account-block type (see [`reference/glossary.md`](../reference/glossary.md)). |

## Signature scheme

When an endpoint has a `secret`, every `POST` to that endpoint carries:

```
X-Webhook-Signature: <hex>
```

where `<hex>` is the lowercase hex encoding of
`HMAC-SHA256(secret, rawBody)` — the HMAC is computed over the **exact
raw JSON request body**, keyed by the endpoint's configured secret. There
is no timestamp or version prefix; the header value is just the hex digest.

To verify, recompute the HMAC over the bytes you received (do not
re-serialize the parsed JSON — whitespace differences would change the
digest) and compare against the header using a constant-time comparison.

### Verification example (Node.js)

```js
const crypto = require("crypto");

function verify(secret, rawBody, header) {
  const expected = crypto
    .createHmac("sha256", secret)
    .update(rawBody)           // rawBody is the exact received bytes
    .digest("hex");
  // constant-time compare
  return (
    header.length === expected.length &&
    crypto.timingSafeEqual(Buffer.from(header), Buffer.from(expected))
  );
}
```

### Verification example (Python)

```python
import hmac, hashlib

def verify(secret: str, raw_body: bytes, header: str) -> bool:
    expected = hmac.new(secret.encode(), raw_body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, header)
```

Reject any request whose signature does not match, and reject unsigned
requests on endpoints you configured with a secret.

## Delivery semantics

Delivery is **best-effort with no delivery guarantee**: events can be **lost**
(crash or backpressure) and can be **duplicated** (re-sync/backfill). There is
no durable outbox. Design consumers to tolerate both — be **idempotent**
(deduplicate on `hash`, or `height` for momentums) and don't assume every
on-chain event produces exactly one webhook. If you need guaranteed delivery,
poll the REST API / database as the source of truth and treat webhooks only as
a low-latency hint.

- **Fire-and-forget.** Emitting an event never blocks the sync loop. Events
  are placed on a bounded in-memory queue (1024 entries) drained by a single
  worker goroutine.
- **Loss on crash (no replay for normal sync).** Events are emitted *after*
  the momentum's DB transaction commits. If the process crashes between commit
  and enqueue/delivery, those events are **lost**: live sync resumes from the
  committed DB height and does **not** re-process it, so it won't re-fire them.
- **Drop under backpressure (unrecoverable).** If the queue is full (a slow or
  unreachable endpoint stalling the worker), new events are **dropped** with a
  warning and never retried.
- **Bounded retries.** A failed delivery (network error or non-2xx response)
  is retried up to `max_retries` times with a short linear backoff, then
  given up on with a log line.
- **Duplicates on replay.** The same per-momentum path runs during
  **backfill** and any re-sync of already-indexed heights, so those heights
  re-fire their events. This is the one case where an event is delivered more
  than once — hence the idempotency requirement above.
- **No strong ordering.** Across endpoints there is no ordering guarantee.
  Within a single endpoint deliveries are roughly in-order (single worker),
  but retries and drops mean strict ordering is **not** guaranteed. The
  `account_block.inserted` events for a momentum are emitted after that
  momentum's `momentum.inserted`, but don't rely on this under failure.

Because the queue is in-memory and unpersisted, events queued but not yet
delivered when the indexer stops are lost. A graceful stop lets the
in-flight delivery (and its retry loop) finish, but does not flush the
remaining queue. A durable outbox/replay path would be required for
at-least-once delivery; that is intentionally out of scope for this version.

## Security

Endpoint **secrets live in plaintext** in `config.yaml` — they are *not*
env-injected. Treat the config file as sensitive:

- Restrict file permissions (e.g. `chmod 600 config.yaml`) and limit who
  can read it.
- **Never commit real secrets.** `config.yaml` is gitignored
  ([`config.yaml.example`](https://github.com/0x3639/nom-indexer-go/blob/main/config.yaml.example)
  is the committed template); keep production secrets out of version control.
- Use a unique, high-entropy secret per endpoint so a leak is scoped to one
  subscriber, and rotate it by editing `config.yaml` and restarting the
  indexer.
- Prefer HTTPS endpoint URLs so the body and signature header aren't sent in
  the clear.
