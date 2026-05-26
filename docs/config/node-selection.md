---
title: Node selection
---

# Node selection

The indexer requires a WebSocket-enabled Zenon node. The default
public test node is `wss://test.hc1node.com`.

## What the indexer needs from a node

| API surface | Used by |
|---|---|
| `LedgerApi.GetFrontierMomentum` | Sync cursor + frontier check. |
| `LedgerApi.GetMomentumsByHeight` | Initial sync, catch-up sync, backfill. |
| `LedgerApi.GetAccountBlockByHash` | Per-account-block fetch inside `processMomentum`. |
| `LedgerApi.GetAccountInfoByAddress` | Per-touched-address balance refresh. |
| `SubscriberApi.ToMomentums` | Real-time subscription after initial sync. |
| `PillarApi.GetAll` | Cached pillar sync (5 min). |
| `SentinelApi.GetAllActive` | Cached sentinel sync (5 min). |
| `AcceleratorApi.GetAll` | Cached project sync (5 min). |
| `BridgeApi.GetBridgeInfo` / `GetSecurityInfo` / `GetOrchestratorInfo` / `GetAllNetworks` / `GetAllWrapTokenRequests` / `GetAllUnwrapTokenRequests` | Bridge sync (1 min). |

Any node serving these RPCs will work. Read-only — the indexer never
issues state-changing calls.

## WebSocket vs HTTP

The SDK supports both, but this indexer uses WS exclusively. WS is
required for the momentum subscription (HTTP polling would miss
real-time blocks). On reconnect, the SDK handles connection lifecycle
transparently — see
[`docs/architecture/sync-and-recovery.md`](../architecture/sync-and-recovery.md).

## Choosing a node

| Option | When to use |
|---|---|
| Public test node (`wss://test.hc1node.com`) | Development, demos, integration tests. Slow under load. |
| Self-hosted alphanet node | Production. Set `NODE_URL_WS=ws://your-node:35998`. |
| Compose-bundled local znnd | Self-host via the `local-node` compose profile — `znnd` is built from source and snapshot-bootstrapped automatically. Indexer points at `ws://znnd:35998`. See [`operations/znnd-bootstrap.md`](../operations/znnd-bootstrap.md). |
| Public mainnet community node | Only if you trust the operator — the indexer trusts every response. |

For production, run your own. Indexing assumes the node is honest;
indexing into a tampered node produces tampered data.

## Latency expectations

Per-momentum processing on a healthy connection:

| Step | Typical | Slow-node case |
|---|---|---|
| `GetMomentumsByHeight` (single momentum) | 5–50ms | 500ms+ |
| `GetAccountBlockByHash` per block | 5–50ms | 500ms+ |
| `GetAccountInfoByAddress` per address | 5–50ms | 500ms+ |
| `AcceleratorApi.GetAll` full walk | 60–120s | 5min+ |

The accelerator walk is the dominant tax on cached-data sync. See
[`docs/operations/failure-modes.md`](../operations/failure-modes.md).

## What breaks when the node is behind

- The sync cursor catches up and stalls at the node's frontier.
- Newly created blocks won't appear until the node catches up.
- `MAX(height)` will stop advancing — that's the canonical "is the node
  healthy?" check (see [`docs/operations/monitoring.md`](../operations/monitoring.md)).

The indexer does **not** verify block validity beyond what the node
returns; if the node is on a stuck fork, the indexer follows the node.

## Switching nodes mid-run

Stop the indexer, change `NODE_URL_WS`, start the indexer. Sync will
resume from the DB's `MAX(height)`. If the new node has a different
chain head, you'll likely want to run `cmd/backfill` first to make sure
no momentums on the old chain remain in DB without being present on the
new one.
