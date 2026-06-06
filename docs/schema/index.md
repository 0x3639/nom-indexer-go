---
title: Schema reference
---

# Schema reference

This is the canonical contract between the indexer and its consumers. Every
table has a dedicated page using the same template (Purpose, Columns, Primary
key & indexes, Relations, Write path, Read patterns, Gotchas).

Before drilling into a specific table, read [conventions](conventions.md) —
the int64-cap rule, timestamp encoding, hash encoding, and the
no-foreign-keys design apply uniformly.

## By domain

### Core ledger

| Table | What it holds |
|---|---|
| [`momentums`](momentums.md) | Block headers indexed by height. |
| [`account_blocks`](account_blocks.md) | Every transaction with decoded ABI inputs. |
| [`accounts`](accounts.md) | One row per address; flow metrics, delegation, genesis seed. |
| [`balances`](balances.md) | Current balance per (address, token). |
| [`tokens`](tokens.md) | ZTS token registry with current supply + holder/tx counts. |
| [`token_mints`](token_mints.md) | Every mint event as its own row. |
| [`token_burns`](token_burns.md) | Every burn event as its own row. |

### Pillars and delegation

| Table | What it holds |
|---|---|
| [`pillars`](pillars.md) | Current pillar registry. |
| [`pillar_updates`](pillar_updates.md) | Append-only history of pillar config changes. |
| [`delegations`](delegations.md) | Time-bucketed delegator → pillar intervals. |

### Sentinels, stakes, plasma

| Table | What it holds |
|---|---|
| [`sentinels`](sentinels.md) | Sentinel node registrations. |
| [`stakes`](stakes.md) | Staking entries (with ABI-derived `cancel_id`). |
| [`fusions`](fusions.md) | Plasma fusion entries (with ABI-derived `cancel_id`). |
| [`htlcs`](htlcs.md) | Hash-time-locked contract entries (Create → Unlock/Reclaim). |

### Accelerator-Z

| Table | What it holds |
|---|---|
| [`projects`](projects.md) | Accelerator-Z funding projects. |
| [`project_phases`](project_phases.md) | Project phases (sub-grants). |
| [`votes`](votes.md) | Pillar votes on projects/phases. |

### Rewards

| Table | What it holds |
|---|---|
| [`reward_transactions`](reward_transactions.md) | Per-event reward receipts. |
| [`cumulative_rewards`](cumulative_rewards.md) | Running total per (address, type, token). |

### Bridge

| Table | What it holds |
|---|---|
| [`wrap_token_requests`](wrap_token_requests.md) | ZTS → external-chain wrap intents. |
| [`unwrap_token_requests`](unwrap_token_requests.md) | External-chain → ZTS unwrap intents. |
| [`bridge_networks`](bridge_networks.md) | Configured destination networks (paginated from BridgeApi). |
| [`bridge_network_tokens`](bridge_network_tokens.md) | Per-network token pair configuration (fees, min, redeem delay). |
| [`bridge_admin`](bridge_admin.md) | Singleton with current administrator + halt state. |
| [`bridge_guardians`](bridge_guardians.md) | Active guardian set. |
| [`bridge_orchestrator_info`](bridge_orchestrator_info.md) | Singleton with orchestrator parameters. |
| [`bridge_security_info`](bridge_security_info.md) | Singleton with security delay parameters. |

### Daily snapshots

| Table | What it holds |
|---|---|
| [`network_stat_histories`](network_stat_histories.md) | Daily network-wide totals + activity. |
| [`token_stat_histories`](token_stat_histories.md) | Daily per-token mints/burns + carried state. |
| [`pillar_stat_histories`](pillar_stat_histories.md) | Daily per-pillar weight + delegator count. |
| [`bridge_stat_histories`](bridge_stat_histories.md) | Daily per-(network, chain, token) wrap/unwrap volume. |

## Where rows come from

- Most rows are written by `processMomentum` in
  [`internal/indexer/processor.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go),
  inside the transactional batch for that height. Each table's page names the
  exact write site.
- Bridge config tables are refreshed by `updateBridgeConfig` in
  [`internal/indexer/indexer.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/indexer.go),
  running on a 1-minute cadence.
- The `_stat_histories` tables are written by the cron loop in
  [`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go)
  on a 1-hour cadence.

See [Architecture overview](../architecture/overview.md) for the broader
goroutine layout.

## Generated column reference

[`_generated.md`](_generated.md) is reserved for an optional `tbls`
appendix generated from a freshly migrated database. In this checkout it is
a status page with the regeneration command; the hand-written per-table
pages and the SQL migrations are the canonical references.
