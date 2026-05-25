---
title: Migration history
---

# Migration history

Narrative of every migration with the problem it solved.

## 001 — `initial_schema`

The original 15-table schema ported from the Dart indexer: `momentums`,
`accounts`, `balances`, `account_blocks`, `tokens`, `pillars`,
`pillar_updates`, `sentinels`, `stakes`, `projects`, `project_phases`,
`votes`, `fusions`, `cumulative_rewards`, and `reward_transactions`.

Design choices baked in here that all later work inherits:

- No foreign keys (see
  [`schema/conventions.md`](../schema/conventions.md#foreign-keys)).
- Hex-string IDs for naturally-keyed tables; `SERIAL`/`BIGSERIAL` for
  event tables without a natural hash.
- `BIGINT` for every numeric stored as `*big.Int` (the int64 cap rule
  that the rest of the schema honors).
- Empty-string-not-NULL discipline for TEXT columns.

## 002 — `indexes`

Adds performance indexes on every column the indexer's queries
filter or join on (`account_blocks.address`, `…to_address`, …;
`pillars.name`; `balances.balance > 0` partial index; etc.). No data
changes; safe to re-run.

## 003 — `bridge_requests`

Adds `wrap_token_requests` and `unwrap_token_requests`. Originally
these track bridge requests without finality progress fields; those
land in 004.

## 004 — `bridge_finality_fields`

Adds `confirmations_to_finality` to `wrap_token_requests` and
`redeemable_in` to `unwrap_token_requests`. `redeemed` and `revoked`
were already present in 003. The new fields let the bridge sync
distinguish finalized rows from in-flight ones — without this, the
indexer had to refetch every wrap on every tick.

## 005 — `fix_log_index_type`

Widens `unwrap_token_requests.log_index` from `INT` to `BIGINT`. Some
external chains emit log indices that overflow int32 on busy blocks.

## 006 — `deduplicate_votes`

Adds the unique constraint
`UNIQUE (voter_address, voting_id)` to `votes` and deletes duplicate
rows that accumulated under the original insert-always behavior.
Backstop for the indexer's per-vote upsert logic — without the
constraint, a brief race could have left two rows for the same
(voter, proposal) pair.

## 007 — `token_mint_burn_events`

Adds [`token_mints`](../schema/token_mints.md) and
[`token_burns`](../schema/token_burns.md) as event-keyed tables. Closes
a gap against the zenonhub PHP indexer (we previously tracked only a
running burn counter on `tokens.total_burned`). Lets consumers query
"every mint by issuer X to receiver Y in date range D" without joining
through `account_blocks`.

Historical mint/burn event backfill is not bundled today; new rows are
captured by the live indexer from this migration forward.

## 008 — `bridge_config`

Adds bridge configuration cache tables:
[`bridge_networks`](../schema/bridge_networks.md),
[`bridge_network_tokens`](../schema/bridge_network_tokens.md),
[`bridge_admin`](../schema/bridge_admin.md),
[`bridge_guardians`](../schema/bridge_guardians.md),
[`bridge_orchestrator_info`](../schema/bridge_orchestrator_info.md),
[`bridge_security_info`](../schema/bridge_security_info.md). Mirrors the
zenonhub bridge schema so the API and MCP server can answer "what's the
current bridge admin?" without an RPC round-trip.

## 009 — `account_flow_and_balance_timestamp`

Extends [`accounts`](../schema/accounts.md) with eight new columns:
genesis ZNN/QSR balance, running ZNN/QSR sent/received totals, and
`first_active_at` / `last_active_at`. Adds
`balances.last_updated_timestamp`.

Enables flow-style queries the original Dart indexer couldn't answer
("who's currently active?", "lifetime ZNN received by this address",
"genesis allocation breakdown").

## 010 — `daily_stat_histories`

Adds the four daily snapshot tables:
[`network_stat_histories`](../schema/network_stat_histories.md),
[`token_stat_histories`](../schema/token_stat_histories.md),
[`pillar_stat_histories`](../schema/pillar_stat_histories.md),
[`bridge_stat_histories`](../schema/bridge_stat_histories.md).

Pairs with the new 1-hour `runStatSnapshots` cron job. Lets the
explorer render time-series without scanning the source tables for
every chart.

## 011 — `delegation_history`

Adds [`delegations`](../schema/delegations.md) as time-bucketed
intervals. Replaces the original "current delegation only on
`accounts.delegate`" model (kept for backward compat). Enables
historical attribution queries — "what was X delegated to on date D?",
"how many delegators did pillar P have on D?".

## 012 — `account_seen_and_tx_count`

Adds `accounts.first_seen`, `accounts.last_seen`, and
`accounts.tx_count`. Maintained incrementally by `BumpTxCountBatch`
in [`internal/repository/account.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account.go),
called once per role the address plays in each indexed block (sender
and recipient, deduped for self-sends).

The migration backfills all three from the existing `account_blocks`
table in a single `INSERT … ON CONFLICT DO UPDATE`. Stub account rows
are created for addresses that only appear as `to_address` (recipients
of sends that have not yet been claimed) so their counters survive
a future `GET /accounts/{address}` lookup.

Distinct from the older `first_active_at` / `last_active_at` pair
(chain-owner blocks only) and from `block_count` (sender-only chain
height). `tx_count` matches `pagination.total` from
`/api/v1/accounts/{address}/transactions`.

## What's next

No migration is currently in flight. The next likely candidates,
documented in their respective schema pages as future work:

- A `tokens.created_at` column so `network_stat_histories.daily_tokens`
  stops being permanently zero.
- A backfill script for `delegations` pre-011 history.
- A `pillar_stat_histories` reward attribution pass to populate
  `momentum_rewards` / `delegate_rewards`.
