# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this repository.

## Project Overview

nom-indexer-go is a Go port of the Dart-based nom-indexer for the Zenon Network blockchain. It indexes blockchain data (momentums, account blocks, embedded contract events) into PostgreSQL for querying.

## Architecture

```
cmd/indexer/main.go          - Entry point
internal/
  config/config.go           - Configuration via Viper (env vars + YAML)
  database/
    database.go              - PostgreSQL connection pool (pgx)
    migrations.go            - golang-migrate integration
  indexer/
    indexer.go               - Main indexer loop, sync logic, cached data
    processor.go             - Momentum and account block processing
    decoder.go               - ABI decoding for embedded contracts
    embedded.go              - Embedded contract event handlers
    rewards.go               - Reward tracking logic
  models/
    models.go                - Database models
    constants.go             - Contract addresses, token standards
  repository/                - Database repositories (one per table)
migrations/                  - SQL migration files
reference/                   - Original Dart nom-indexer code for reference
  bin/indexer/nom_indexer.dart - Main indexer logic to compare against
```

## Key Dependencies

- `github.com/0x3639/znn-sdk-go` - Zenon SDK for RPC communication
- `github.com/jackc/pgx/v5` - PostgreSQL driver with connection pooling
- `github.com/golang-migrate/migrate/v4` - Database migrations
- `github.com/spf13/viper` - Configuration management
- `go.uber.org/zap` - Structured logging

## Common Commands

```bash
# Build locally
GOWORK=off go build ./...

# Run with Docker
docker-compose up -d --build

# View logs
docker logs nom-indexer -f

# Check database
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer -c "SELECT MAX(height) FROM momentums;"

# Stop containers
docker-compose down

# Rebuild after code changes
docker-compose up -d --build
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| NODE_URL_WS | WebSocket URL for Zenon node | wss://test.hc1node.com |
| DATABASE_ADDRESS | PostgreSQL host | localhost |
| DATABASE_PORT | PostgreSQL port | 5432 |
| DATABASE_NAME | Database name | nom_indexer |
| DATABASE_USERNAME | Database user | postgres |
| DATABASE_PASSWORD | Database password | - |
| MIGRATIONS_PATH | Path to migrations folder | migrations |

## Known Issues & Workarounds

### Balance Updates Performance
Balance updates are skipped for momentums with >1000 transactions (like genesis) because fetching account info for each address individually is slow.

**Location**: `internal/indexer/processor.go` - `processMomentum()` function

### SDK Accelerator Types Fix (In Progress)
The Go SDK was reusing go-zenon's internal types for client-side JSON deserialization, which caused a panic when parsing accelerator projects with phases. The Dart SDK correctly defines its own client-side types.

**Fix**: Created new SDK types in `api/embedded/accelerator_types.go` with proper JSON unmarshaling.

**Status**:
- SDK fix is in `/Users/dfriestedt/Github/zenon-go-sdk/znn-sdk-go/api/embedded/accelerator_types.go`
- Indexer currently uses a local `vendor-sdk/` copy with a `replace` directive in go.mod
- **TODO**: After SDK is published with new version, update go.mod to remove the replace directive and vendor-sdk folder

## API Documentation References

- Ledger API: https://docs.0x3639.com/developer/rpc-api/core/dual-ledger
- Subscribe API: https://docs.0x3639.com/developer/rpc-api/core/subscribe
- Embedded Contracts: https://docs.0x3639.com/developer/rpc-api/embedded/

## Docker Notes

- Uses Go 1.24 (required by znn-sdk-go which needs Go 1.24+)
- CGO is enabled (required for secp256k1 crypto operations in go-zenon)
- Multi-stage build: golang:1.24-alpine for build, alpine:3.19 for runtime

## Database Schema

15 tables tracking:
- `momentums` - Block headers
- `account_blocks` - Transactions
- `balances` - Token balances per address
- `pillars` - Pillar nodes
- `sentinels` - Sentinel nodes
- `stakes` - Staking entries
- `fusions` - Plasma fusion entries
- `delegations` - Delegation records
- `tokens` - ZTS tokens
- `projects` - Accelerator-Z projects
- `project_phases` - Project phases
- `votes` - Pillar votes
- `rewards` - Reward distributions
- `plasma_events` - Plasma fuse/cancel events
- `token_events` - Token mint/burn events
