---
title: Schema conventions
---

# Schema conventions

These rules apply across every table in this schema. Where a per-table page
notes a column behavior, it links back here so the rule is described in exactly
one place.

The schema is the contract between the indexer and its consumers (human SQL,
the REST API, and the MCP server). Treat any deviation from these rules as a
public-API change.

## int64 cap

Every amount, balance, supply, fee, weight, or other numeric value derived from
a Zenon `*big.Int` is stored as `BIGINT` (signed int64).

The indexer converts `*big.Int` → `int64` through a single helper,
[`safeBigIntToInt64`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go).
If the source value exceeds `math.MaxInt64` (≈9.22 × 10¹⁸), the helper logs a
warning and returns `math.MaxInt64`. **Values are silently capped, not rejected
and not truncated to lower bits.**

In practice:

- ZNN and QSR amounts use 1e8 satoshi scaling. Total supplies are well below
  the cap (network supply ≈ 1.5 × 10¹⁵ satoshi).
- Any custom ZTS token whose supply approaches 9.22 × 10¹⁸ satoshi will be
  capped in `tokens.total_supply` / `tokens.max_supply`. The indexer logs a
  warning when this happens; consumers should treat capped rows as suspect.

If the project ever needs full-precision values, the columns can be migrated
to `NUMERIC(78,0)` without changing the row-level interface — but this has
not happened yet.

## Timestamps

All timestamp columns are **Unix seconds in `BIGINT`** (UTC). The schema does
not use `TIMESTAMP` or `TIMESTAMPTZ` anywhere. Comparing against a calendar day
must convert explicitly:

```sql
WHERE momentum_timestamp >= EXTRACT(EPOCH FROM ('2026-01-15'::date AT TIME ZONE 'UTC'))::bigint
  AND momentum_timestamp <  EXTRACT(EPOCH FROM (('2026-01-15'::date + INTERVAL '1 day') AT TIME ZONE 'UTC'))::bigint
```

This avoids session-timezone surprises. The cron snapshot queries use this
form throughout.

## Hash encoding

Hashes (block hashes, momentum hashes, voting IDs, cancel IDs) are stored as
**64-character lowercase hexadecimal strings** in `TEXT`. No `0x` prefix, no
leading whitespace, exact length 64.

Addresses are stored as bech32-encoded `z1…` strings. Token standards are
stored as `zts1…` strings. Constants for the special embedded-contract
addresses live in [`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go);
see also [`docs/reference/addresses.md`](../reference/addresses.md).

## Empty-string-not-NULL

Most `TEXT` columns are declared `NOT NULL DEFAULT ''`. This intentionally
avoids tri-state semantics — empty string means "no value" or "not applicable".
Comparing against NULL in queries is a sign the consumer is reading a column
that shouldn't be tested that way; check the per-table page for the column's
"empty" meaning.

A handful of columns explicitly use `NULL` because absence is meaningful:

- `accounts.first_active_at` / `accounts.last_active_at` — NULL means the
  account has never been observed in a block.
- `delegations.ended_at` — NULL means the delegation is currently active.

These exceptions are called out on the relevant per-table pages.

## Identifiers

Two kinds of primary keys are used:

| Pattern | When | Examples |
|---|---|---|
| Hash string (`TEXT` 64-char) | When the row corresponds 1:1 with a blockchain object that already has a hash ID. | `account_blocks.hash`, `stakes.id`, `fusions.id`, `projects.id`, `project_phases.id`. |
| `SERIAL` / `BIGSERIAL` autoincrement | When the row records an event or rollup with no natural hash ID. | `pillar_updates.id`, `votes.id`, `cumulative_rewards.id`, `delegations.id`, `token_mints.id`, `token_burns.id`. |

Composite keys are used where the natural unique tuple is multi-column:
`balances (address, token_standard)`, `unwrap_token_requests (transaction_hash, log_index)`,
`bridge_network_tokens (network_class, chain_id, token_standard)`, etc.

## Foreign keys

The schema **does not declare any FOREIGN KEY constraints**. Relations are by
convention (matching column name + type), not by enforcement. Two reasons:

1. The indexer writes batched inserts within a transaction; foreign-key
   verification per-row would force per-row work where batching wins.
2. Out-of-order processing can briefly produce orphan rows (an account block
   referencing a momentum that lands in the same batch). The transaction is
   the unit of atomicity, not individual FK checks.

Where two tables relate by convention, the relation is documented on each
table's "Relations" section in the per-table page, and visualized in
[`erd.md`](erd.md).

## Singleton tables

Three bridge-config tables use a `row_id SMALLINT CHECK (row_id = 1) PRIMARY KEY`
pattern to enforce a single row: `bridge_admin`, `bridge_orchestrator_info`,
`bridge_security_info`. Consumers read them as `WHERE row_id = 1` (or just
`LIMIT 1`).

## JSONB

Two columns use `JSONB`:

- `account_blocks.input` — decoded ABI inputs as a flat string→string map.
  Sanitized to remove `\u0000` escapes and literal NUL bytes before insert
  (PG rejects NUL in JSONB).
- (Reserved for future expansion in stat-history `meta` columns; not used
  today.)

Querying with `->>` returns the input value as text.

## Batch writes and idempotency

Most repository writes are batched inside `processMomentum`'s pgx transaction.
On batch failure the whole momentum's writes roll back and the sync loop
retries the height — so per-row inserts should be idempotent under retry:

- Tables whose PK is content-derived (hashes) use `ON CONFLICT … DO NOTHING`.
- Tables that mutate existing rows (pillars, tokens, accounts) use
  `ON CONFLICT … DO UPDATE SET …`.
- A handful of additive counters (`cumulative_rewards.amount`) use
  `DO UPDATE SET amount = existing + EXCLUDED` — these are **not** safe to
  re-run blindly outside the original transaction. The one-shot backfill
  scripts in [`scripts/`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts)
  must use `ON CONFLICT (hash) DO NOTHING` on the event-keyed table and rely
  on the `RowsAffected()` skip to avoid double-counting.

## When this page changes

A change to any rule on this page is a schema-wide breaking change. Update
the migration history in [`migrations/history.md`](../migrations/history.md)
with the reason and the version at which the rule changed.
