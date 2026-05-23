---
title: FAQ
---

# FAQ

Common questions and the queries / files that answer them.

## Why is `balances` stale for some addresses?

`updateBalances` skips momentums with ≥1000 transactions (the
`genesisBalanceUpdateThreshold` constant). Genesis is the obvious
case. Use [`accounts.genesis_znn_balance`](../schema/accounts.md) /
`genesis_qsr_balance` for genesis seeds, and the live RPC for current
balances on rarely-touched addresses.

## Why does `tokens.holder_count` lag?

Refreshed by the cron loop on `cron.token_holders_interval` (default
10 min). For real-time counts, query [`balances`](../schema/balances.md)
directly:

```sql
SELECT COUNT(*) FROM balances
WHERE token_standard = $1 AND balance > 0;
```

## Why are my `*big.Int` amounts truncated?

The schema stores them as `BIGINT` (int64). Values above
`math.MaxInt64` (≈9.22 × 10¹⁸) are capped through `safeBigIntToInt64`
with a warning log. See
[`schema/conventions.md`](../schema/conventions.md#int64-cap).

## How do I query rewards for an address?

Two tables:

- [`reward_transactions`](../schema/reward_transactions.md) — per-event
  rows with momentum height, amount, source contract.
- [`cumulative_rewards`](../schema/cumulative_rewards.md) — running
  total per (address, reward type, token).

```sql
SELECT reward_type, amount, token_standard
FROM cumulative_rewards
WHERE address = 'z1q…';

SELECT * FROM reward_transactions
WHERE address = 'z1q…'
ORDER BY momentum_height DESC LIMIT 100;
```

## Why doesn't my pillar have `voting_activity`?

Either:

- The pillar was just spawned and no eligible proposals exist since
  spawn (`voting_activity = 0` is the correct value), or
- The cron hasn't run since spawn — wait up to
  `cron.voting_activity_interval` (default 10 min).

## How do I find all account blocks for an address?

```sql
SELECT * FROM account_blocks
WHERE address = $1 OR to_address = $1
ORDER BY momentum_height DESC;
```

Both columns are indexed.

## My indexer is caught up but `gaps > 0`. What's going on?

A previous indexer run was stopped mid-sync. Run the backfill — see
[`docs/operations/backfill.md`](../operations/backfill.md).

## What does `RewardTypeUnknown` (0) mean?

The `classifyReward` function couldn't categorize the reward — the
source address didn't match any known reward contract. Rows with this
type are rare and indicate either a new reward contract or a bug in
classification.

## Why is `votes.phase_id = ''` instead of NULL?

By convention. Tests using `phase_id IS NULL` will never match. Use
`phase_id = ''` for "project-level vote". See
[`schema/conventions.md`](../schema/conventions.md#empty-string-not-null).

## Why are bridge guardians never deleted?

Removed guardians have `nominated = false AND accepted = false`, set
by the `MarkGuardiansAbsent` sweep that runs each bridge sync tick.
The row stays for historical reference. See
[`schema/bridge_guardians.md`](../schema/bridge_guardians.md).

## How do I add a new contract handler / table / cron job?

See the recipes in [`docs/development/`](../development/setup.md):

- [Add a contract handler](../development/add-contract-handler.md)
- [Add a table](../development/add-table.md)
- [Add a cron job](../development/add-cron-job.md)

## Where does the future REST API fit?

It will live alongside the indexer (`cmd/api`) and read from the same
Postgres. The schema is the contract. See
[`docs/api/index.md`](../api/index.md) for the stub and
[`specs/API_SPECIFICATION.md`](https://github.com/0x3639/nom-indexer-go/blob/main/specs/API_SPECIFICATION.md)
for the draft implementation brief.

## Where does the future MCP server fit?

Same pattern: `cmd/mcp`, reads from Postgres, exposes resources +
tools by table. See [`docs/mcp/index.md`](../mcp/index.md).

## How do I know if the indexer is healthy?

```sql
SELECT MAX(height), extract(epoch from now()) - MAX(timestamp) AS seconds_behind
FROM momentums;
```

`seconds_behind < 30` after initial catch-up. See
[`docs/operations/monitoring.md`](../operations/monitoring.md).

## How big does the database get?

Estimates:

| Chain age | Approx data size |
|---|---|
| Test net at 13M momentums | ~5 GB |
| Mainnet projection (~30M) | ~15–20 GB |

`account_blocks` dominates. See
[`docs/operations/scaling.md`](../operations/scaling.md).
