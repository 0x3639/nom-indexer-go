---
title: Known issues
---

# Known issues

Canonical list of known limitations and historical bugs that left
fingerprints in the data. Cross-referenced from
[`operations/failure-modes.md`](../operations/failure-modes.md) and
the per-table schema pages.

## int64 cap on `*big.Int` values

**What:** Amounts > `math.MaxInt64` (≈9.22 × 10¹⁸) are silently capped
to `math.MaxInt64` by `safeBigIntToInt64`. A warning is logged.

**Why:** Schema columns are `BIGINT`. Switching to `NUMERIC(78,0)`
would lose `bigint`-arithmetic ergonomics in SQL and break clients
that read into `int64`. Not changing today.

**Affected:** Any column that comes from a `*big.Int` —
`*.amount`, `*.weight`, `tokens.{total_supply, max_supply}`,
`balances.balance`. ZNN and QSR amounts are well below the cap;
custom ZTS tokens that approach 9.22e18 satoshi will be capped.

**Detection:** Search for "overflow" in the indexer logs, or
`SELECT * FROM tokens WHERE total_supply = 9223372036854775807`.

**Mitigation:** None today. Capping is the documented behavior.

## Balance updates skipped on big momentums

**What:** Momentums with ≥1000 account blocks (genesis, hypothetical
spike blocks) skip per-address balance refresh.

**Why:** `GetAccountInfoByAddress` is one RPC call per address. At
genesis, that's tens of thousands of round-trips — would take hours.

**Affected:** `balances` rows for addresses funded only at genesis;
the flow-tracking columns in `accounts` (which are computed from
sends/receives) remain accurate.

**Mitigation:** None today. See
[`schema/balances.md`](../schema/balances.md).

## Reward indexing pre-fix BlockType bug

**What:** For most of the indexer's lifetime, the reward-detection
branch used literal BlockType ints (`4` for "UserReceive", `6` for
"ContractSend") that didn't match the SDK's enum values. The branch
never fired; `reward_transactions` for non-liquidity rewards was
empty.

**Why:** The Dart port translated `BlockTypeEnum.userReceive.index`
incorrectly because the Dart enum positions differ from the Go SDK
constants.

**Fix:** Replaced with `utils.BlockTypeUserReceive` (3) and
`utils.BlockTypeContractSend` (4). Historical data is repopulated by
[`scripts/backfill-rewards`](https://github.com/0x3639/nom-indexer-go/tree/main/scripts/backfill-rewards).

**Affected:** Any DB that ran the indexer before the fix. Detection:
`SELECT reward_type, COUNT(*) FROM reward_transactions GROUP BY
reward_type` — should show entries for types 1, 3, 4, 5 (and now 2
post-classification fix).

## Historical SDK accelerator-types JSON panic

**What:** The upstream `znn-sdk-go` reused go-zenon's internal types
for client-side JSON deserialization, which caused a panic when
parsing accelerator projects with phases.

**Why:** go-zenon's accelerator type uses a different JSON shape on
the client side than on the chain side; the SDK's reuse broke the
boundary.

**Fix:** The current checkout uses upstream
`github.com/0x3639/znn-sdk-go` directly; there is no `vendor-sdk/`
directory or `replace` directive in `go.mod`.

**Status:** Historical issue. If a future SDK bump brings this panic
back, pin or roll back `github.com/0x3639/znn-sdk-go`, open an
upstream issue, and document the temporary workaround here.

## Bridge unwrap "unknown network" warnings

**What:** Recurring WARN log lines like `bridge sync: failed to update
unwrap requests {"error": "unknown network"}`.

**Why:** The test-network bridge API returns this for certain network
classes the indexer hasn't been configured to ignore.

**Affected:** Test-network deployments. Wraps still sync; only
unwraps miss the refresh on the affected ticks.

**Mitigation:** None today — the indexer logs the warning and
continues. The error doesn't propagate.

## `pillar_stat_histories.momentum_rewards` / `delegate_rewards` not populated

**What:** Both columns exist in the schema and are written by the
cron, but the values are always `0`.

**Why:** Populating them correctly requires joining
[`reward_transactions`](../schema/reward_transactions.md) against
[`delegations`](../schema/delegations.md) history to attribute
delegate rewards to the right pillar at the right time — non-trivial
SQL that hasn't been written.

**Status:** Not implemented; see the `snapshotPillarStats` comment in
[`internal/indexer/cron.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/cron.go).

## `network_stat_histories.daily_tokens` always 0

**What:** The column never reflects per-day token creation.

**Why:** [`tokens`](../schema/tokens.md) has no `created_at` column.
Adding one is a migration plus a write-path change; not done today.

## `delegations` pre-migration-011 history is missing

**What:** Migration 011 introduced the
[`delegations`](../schema/delegations.md) table. Prior delegation
events are recorded as `Delegate`/`Undelegate` rows in
[`account_blocks`](../schema/account_blocks.md) but not as time-bucketed
intervals.

**Status:** A backfill script analogous to `backfill-rewards` is
plausible but not written. The current state (`accounts.delegate`)
remains correct for "who is X delegated to right now?".

## No HTTP `/metrics` endpoint

**What:** The indexer doesn't expose Prometheus metrics.

**Status:** Planned alongside the forthcoming REST API. The
canonical liveness signal today is "`MAX(momentums.timestamp)` is
advancing".

## Dagger CI doesn't run integration tests

**What:** `dagger call test` only runs unit tests.

**Why:** Integration tests need a Postgres sidecar that the current
Dagger module doesn't provision.

**Status:** Plausible follow-up. Locally, run integration tests with
the documented `TEST_DATABASE_URL` flow — see
[`testing/integration-db.md`](../testing/integration-db.md).
