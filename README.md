# nom-indexer-go

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker&logoColor=white)](https://www.docker.com/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A high-performance blockchain indexer for the [Zenon Network](https://zenon.network), written in Go. This is a port of the original [Dart-based nom-indexer](https://github.com/zenon-tools/nom-indexer).

## Overview

nom-indexer-go indexes blockchain data from the Network of Momentum (NoM) into PostgreSQL for efficient querying. It tracks:

- **Momentums** - Block headers with producer information
- **Account Blocks** - All transactions with decoded contract data
- **Balances** - Token balances per address
- **Pillars** - Validator nodes and their configuration history
- **Sentinels** - Sentinel node registrations
- **Stakes** - Staking entries with cancel IDs (computed via ABI encoding)
- **Delegations** - Delegation records
- **Fusions** - Plasma fusion entries with cancel IDs
- **Tokens** - ZTS token metadata and statistics
- **Accelerator-Z** - Projects, phases, and pillar votes (with computed voting IDs)
- **Rewards** - Reward distributions by type
- **Bridge** - Wrap/unwrap token requests with finality tracking

## Quick Start

### Prerequisites

- Docker and Docker Compose
- A Zenon Network node with WebSocket enabled (or use the default public node)

### Running with Docker

```bash
# Clone the repository
git clone https://github.com/0x3639/nom-indexer-go.git
cd nom-indexer-go

# Start the indexer (uses default public node)
docker-compose up -d

# View logs
docker logs nom-indexer -f

# Check indexing progress
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer -c "SELECT MAX(height) FROM momentums;"
```

### Using a Custom Node

```bash
# Set your node's WebSocket URL
export NODE_URL_WS=wss://your-node.example.com:35998

# Start with custom node
docker-compose up -d
```

## Configuration

Configuration can be set via environment variables or `config.yaml`:

| Variable | Description | Default |
|----------|-------------|---------|
| `NODE_URL_WS` | WebSocket URL for Zenon node | `wss://test.hc1node.com` |
| `DATABASE_ADDRESS` | PostgreSQL host | `localhost` |
| `DATABASE_PORT` | PostgreSQL port | `5432` |
| `DATABASE_NAME` | Database name | `nom_indexer` |
| `DATABASE_USERNAME` | Database user | `postgres` |
| `DATABASE_PASSWORD` | Database password | - |
| `MIGRATIONS_PATH` | Path to migrations folder | `migrations` |

## Architecture

The indexer uses independent goroutines for different sync operations to maximize throughput:

```
cmd/
  indexer/main.go            - Entry point
  migrate-cancel-ids/main.go - Migration tool for cancel IDs
internal/
  config/                    - Configuration via Viper (env vars + YAML)
  database/                  - PostgreSQL connection pool (pgx)
  indexer/
    indexer.go               - Main loop with concurrent sync goroutines
    processor.go             - Momentum and block processing
    decoder.go               - ABI decoding for embedded contracts
    embedded.go              - Contract event handlers
    rewards.go               - Reward tracking
  models/                    - Database models
  repository/                - Data access layer (one per table)
migrations/                  - SQL migration files (5 versions)
```

### Sync Architecture

The indexer runs three independent goroutines:

1. **Momentum Subscription** - Real-time processing of new blocks (~10s intervals)
2. **Bridge Sync Loop** - Syncs wrap/unwrap requests every 1 minute
3. **Cached Data Sync Loop** - Updates pillars, sentinels, projects every 5 minutes

This architecture ensures momentum processing is never blocked by slow API calls.

## Database Schema

The indexer creates 17 tables across 5 migrations:

| Table | Description |
|-------|-------------|
| `momentums` | Block headers |
| `account_blocks` | Transactions with decoded method calls |
| `balances` | Current token balances per address |
| `pillars` | Pillar nodes |
| `pillar_updates` | Pillar configuration history |
| `sentinels` | Sentinel registrations |
| `stakes` | Staking entries with cancel_id |
| `delegations` | Delegation records |
| `fusions` | Plasma fusion entries with cancel_id |
| `tokens` | ZTS token registry |
| `projects` | Accelerator-Z projects |
| `project_phases` | Project phases |
| `votes` | Pillar votes on projects |
| `rewards` | Reward distributions |
| `plasma_events` | Plasma fuse/cancel events |
| `token_events` | Token mint/burn events |
| `wrap_token_requests` | Bridge wrap operations |
| `unwrap_token_requests` | Bridge unwrap operations |

## Bridge Sync Logic

The bridge sync uses intelligent paging to minimize API calls:

- **Wrap requests**: Sync from newest to oldest unfinalized TX (wraps finalize sequentially)
- **Unwrap requests**: Sync from newest to oldest unfinalized TX (unwraps finalize out-of-order, user-initiated)

This ensures new transactions are captured while avoiding re-fetching already-finalized data.

## Development

### Building Locally

```bash
# Requires Go 1.24+ with CGO enabled
GOWORK=off go build ./cmd/indexer
```

### Running Tests

```bash
go test ./...
```

### Rebuilding After Changes

```bash
docker-compose up -d --build
```

## Technical Notes

- **Go 1.24** required (dependency requirement from znn-sdk-go v0.1.11)
- **CGO enabled** for secp256k1 cryptographic operations
- **Multi-stage Docker build** for minimal runtime image (~50MB)
- Uses **pgx/v5** with connection pooling for database performance
- Real-time indexing via WebSocket subscriptions after initial sync
- **ABI encoding** for computing voting IDs, stake cancel IDs, and fusion cancel IDs

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| [znn-sdk-go](https://github.com/0x3639/znn-sdk-go) | v0.1.11 | Zenon SDK with ABI encoding |
| [pgx/v5](https://github.com/jackc/pgx) | v5.7.6 | PostgreSQL driver |
| [golang-migrate](https://github.com/golang-migrate/migrate) | v4.19.1 | Database migrations |
| [viper](https://github.com/spf13/viper) | v1.21.0 | Configuration |
| [zap](https://github.com/uber-go/zap) | v1.27.1 | Structured logging |

## API Documentation

For information about the Zenon Network RPC APIs used by this indexer:

- [Ledger API](https://docs.0x3639.com/developer/rpc-api/core/dual-ledger)
- [Subscribe API](https://docs.0x3639.com/developer/rpc-api/core/subscribe)
- [Embedded Contracts](https://docs.0x3639.com/developer/rpc-api/embedded/)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
