---
title: Local znnd + snapshot bootstrap
---

# Local znnd + snapshot bootstrap

The compose stack ships an optional **local-node** profile that bundles a
`znnd` container (built from
[zenon-network/go-zenon](https://github.com/zenon-network/go-zenon)) plus
a one-shot **snapshot bootstrap** init container. Opt in when you want
the indexer to talk to your own node instead of a public one — see
[`config/node-selection.md`](../config/node-selection.md) for the why.

Default `docker compose up -d` does **not** include znnd. You must pass
`--profile local-node` to materialize the two new services.

## Quick start

```bash
# 1. In .env, set:
#      ZNND_BOOTSTRAP_URL=https://snapshots.example/zenon-2026-05-26.zip
#      NODE_URL_WS=ws://znnd:35998
#    The sibling .hash URL (zenon-2026-05-26.hash) must contain the sha256.

# 2. Bring the bundle up. The bootstrap runs first, znnd starts after.
docker compose --profile local-node up -d --build

# 3. Watch each phase. Bootstrap is one-shot; znnd is long-running.
docker compose --profile local-node logs -f znnd-bootstrap
docker compose --profile local-node logs -f znnd

# 4. Once znnd is at chain head, the indexer (already configured against
#    ws://znnd:35998 in step 1) will start filling Postgres.
docker compose logs -f indexer
```

To sync from genesis instead of installing a snapshot, leave
`ZNND_BOOTSTRAP_URL` unset — the bootstrap container will log a "skipping"
message and exit 0, and znnd will sync from block 0 (slow).

## Services

### `znnd-bootstrap` (one-shot)

- Built from `Dockerfile.znnd-bootstrap` (alpine + `wget`, `unzip`,
  `coreutils`). Entrypoint is `scripts/znnd-bootstrap.sh`.
- Mounts the named `znnd_data` volume at `/data`.
- Skips silently when `/data/nom` exists (unless `ZNND_FORCE_BOOTSTRAP=true`)
  or when `ZNND_BOOTSTRAP_URL` is empty.
- Resumable: cache lives at `/data/.bootstrap-cache`; `wget -c` resumes
  partial downloads. The cache is removed only on successful install.
- Verifies sha256 against the sibling `.hash` URL before extracting.
  Corrupt zips (checksum or unzip failure) are deleted so the next run
  redownloads from scratch.
- Validates that all three required snapshot dirs exist before moving
  existing live data aside; incomplete snapshots leave current chain data
  untouched.
- Promotes the snapshot's `backup/{nom,network,consensus}.bak/` dirs to
  `/data/{nom,network,consensus}/`.

### `znnd` (long-running)

- Built from `Dockerfile.znnd`. `golang:1.21-bookworm` builder →
  `debian:bookworm-slim` runtime (CGO required for secp256k1).
- Pinned via `ZNND_GIT_REF` build arg (default `master`). Pin to a tag
  in production.
- `depends_on: znnd-bootstrap (service_completed_successfully)`, so znnd
  only starts after the bootstrap finishes (or no-ops).
- Mounts `znnd_data` at `/root/.znn` and `configs/znnd.config.json` at
  `/root/.znn/config.json` (sets `RPC.HTTPVirtualHosts: ["*"]` so the
  indexer container can reach the WS RPC across the docker network).
- Exposes `35995/tcp+udp` on the host (P2P). RPC (35997) and WS (35998)
  stay internal to the `nom-indexer` docker network.

## Volume layout

The `znnd_data` named volume holds chain data shared between the two
containers:

| Path (in znnd) | Path (in bootstrap) | Contents |
|---|---|---|
| `/root/.znn/nom` | `/data/nom` | Ledger DB. |
| `/root/.znn/network` | `/data/network` | P2P peer state. |
| `/root/.znn/consensus` | `/data/consensus` | Consensus state. |
| `/root/.znn/config.json` | — | Bind-mounted from `configs/znnd.config.json`. |

## Snapshot format

The bootstrap expects a zip containing:

```
backup/
├── nom.bak/
├── network.bak/
└── consensus.bak/
```

…plus a sibling `<snapshot>.hash` URL holding the sha256 of the zip
(first whitespace-separated token; sha256sum's default output format).

## Force a one-time re-bootstrap

Use this to recover from corrupted local chain data without losing the
existing volume contents — the script moves the live dirs aside with a
timestamp suffix rather than deleting them.

```bash
docker compose --profile local-node stop znnd
docker compose --profile local-node run --rm -e FORCE_BOOTSTRAP=true znnd-bootstrap
docker compose --profile local-node start znnd
```

After this completes, inside the `znnd_data` volume you'll see
`nom_<timestamp>/`, `network_<timestamp>/`, `consensus_<timestamp>/`
alongside the freshly installed dirs. Delete the timestamped ones once
the new data syncs cleanly.

## Troubleshooting

### Bootstrap fails with "Checksum mismatch"

The downloaded zip didn't match the published sha256. The script
deletes the corrupt zip and exits non-zero. Rerun the bootstrap
container — `wget -c` will resume any partial download on the next try.
If it keeps mismatching, the publisher's `.hash` may be stale or the
zip on the mirror is corrupted; verify out-of-band before retrying.

### Bootstrap fails with "Zip did not contain expected backup/ subdir"

The snapshot's zip layout doesn't match what the script expects. Either
the publisher changed the layout, or you pointed at the wrong URL.
Inspect the zip manually before retrying.

### Bootstrap fails with "Snapshot is incomplete"

The zip has `backup/` but is missing one or more of
`nom.bak`, `network.bak`, or `consensus.bak`. The script exits before
moving current chain data, so fix the snapshot URL or publisher layout
and rerun.

### znnd healthcheck reports unhealthy

znnd's RPC rejects `Host: localhost` requests with HTTP 403 by design
(whitelist-based). The healthcheck treats "got any HTTP response" as
healthy, so a 403 is fine — but if `curl` can't connect at all
(connection refused / timeout) the daemon is genuinely down.
`docker compose --profile local-node logs znnd` is the first stop.

### Indexer can't reach znnd

If `cmd/indexer` logs websocket dial errors, confirm:

1. `NODE_URL_WS=ws://znnd:35998` in `.env`, **not** `wss://` and not
   `localhost`. The two containers share the docker network and resolve
   each other by service name.
2. `configs/znnd.config.json` is mounted at `/root/.znn/config.json`
   (compose does this for you — verify with `docker compose --profile
   local-node exec znnd cat /root/.znn/config.json`).
3. znnd has actually started serving RPC. On a cold sync from genesis,
   the WS endpoint comes up well before the chain head is reached.

### Port 35995 already in use

Another znnd or a different service holds the host's P2P port.
`docker compose --profile local-node down` your old stack first, or
remove the host port mapping if you don't need inbound P2P.

## Build pinning in production

For reproducible builds, pin `ZNND_GIT_REF` to a release tag rather than
`master`:

```env
ZNND_GIT_REF=v0.0.8
```

Rebuild with `docker compose --profile local-node build znnd` after
changing the ref. The bootstrap container does not need rebuilding when
the snapshot URL changes — that's pure runtime config.
