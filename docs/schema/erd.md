---
title: Entity relationship diagram
---

# Entity relationship diagram

Relations are by convention only — the schema declares **no** foreign-key
constraints. The diagram below shows logical join paths, not enforced
referential integrity. See [conventions](conventions.md#foreign-keys) for
the reasoning.

## Core ledger

```mermaid
erDiagram
    momentums ||--o{ account_blocks : "momentum_hash / momentum_height"
    accounts  ||--o{ account_blocks : "address (sender)"
    accounts  ||--o{ account_blocks : "to_address (recipient)"
    accounts  ||--o{ balances : "address"
    tokens    ||--o{ balances : "token_standard"
    tokens    ||--o{ account_blocks : "token_standard"
    tokens    ||--o{ token_mints : "token_standard"
    tokens    ||--o{ token_burns : "token_standard"
    accounts  ||--o{ token_mints : "issuer / receiver"
    accounts  ||--o{ token_burns : "burner"
```

## Pillars, sentinels, stakes, plasma

```mermaid
erDiagram
    pillars        ||--o{ pillar_updates : "owner_address"
    pillars        ||--o{ delegations    : "pillar_owner_address"
    accounts       ||--o{ delegations    : "delegator_address"
    accounts       ||--|| sentinels      : "owner"
    accounts       ||--o{ stakes         : "address"
    accounts       ||--o{ fusions        : "address / beneficiary"
```

## Accelerator-Z

```mermaid
erDiagram
    projects       ||--o{ project_phases : "project_id"
    projects       ||--o{ votes          : "project_id"
    project_phases ||--o{ votes          : "phase_id"
    pillars        ||--o{ votes          : "voter_address"
```

## Rewards

```mermaid
erDiagram
    accounts ||--o{ reward_transactions : "address"
    accounts ||--o{ cumulative_rewards  : "address"
    tokens   ||--o{ reward_transactions : "token_standard"
    tokens   ||--o{ cumulative_rewards  : "token_standard"
```

## Bridge

```mermaid
erDiagram
    bridge_networks       ||--o{ bridge_network_tokens : "(network_class, chain_id)"
    bridge_networks       ||--o{ wrap_token_requests   : "(network_class, chain_id)"
    bridge_networks       ||--o{ unwrap_token_requests : "(network_class, chain_id)"
    bridge_networks       ||--o{ bridge_stat_histories : "(network_class, chain_id)"
    tokens                ||--o{ wrap_token_requests   : "token_standard"
    tokens                ||--o{ unwrap_token_requests : "token_standard"
    tokens                ||--o{ bridge_stat_histories : "token_standard"
    accounts              ||--o{ wrap_token_requests   : "to_address"
    accounts              ||--o{ unwrap_token_requests : "to_address"
```

## Singleton bridge tables

`bridge_admin`, `bridge_orchestrator_info`, and `bridge_security_info` each
hold exactly one row (`row_id = 1`); they have no incoming references.
`bridge_guardians` is a flat list keyed by address.

## Daily snapshots

`network_stat_histories`, `token_stat_histories`, `pillar_stat_histories`,
and `bridge_stat_histories` are append-on-day-bucketed rollups. Their
relationships to the source tables are date-scoped aggregations, not
row-level joins; see the [cron and snapshots](../architecture/cron-and-snapshots.md)
page for the aggregation queries.
