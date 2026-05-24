# nom-indexer-go API Specification

> **Status: Superseded by [`docs/api/`](../docs/api/index.md). Kept as an ADR.**
>
> The REST API server has shipped. The live reference is the OpenAPI 3.1
> contract at [`docs/api/openapi.yaml`](../docs/api/openapi.yaml) and the
> rendered Swagger UI on the docs site
> ([API overview](https://0x3639.github.io/nom-indexer-go/api/)). The
> per-domain endpoint pages at
> [`docs/api/endpoints/`](../docs/api/endpoints/) cover the same routes
> with curl examples.
>
> This file is retained because it captured the implementation
> decisions (router choice, auth scheme, pagination model, DTO
> strategy) that produced the v1 API. Treat it as an architecture
> decision record, not as the API contract.
>
> For the schema that the REST API reads directly, see
> [`docs/schema/`](../docs/schema/index.md).

This document provides everything needed to build a REST API layer for the nom-indexer-go database using Go and Gin.

## Overview

The nom-indexer-go indexes Zenon Network blockchain data into PostgreSQL. This API layer provides read-only access to the indexed data.

## Technology Stack

- **Framework**: Gin (github.com/gin-gonic/gin)
- **Database Driver**: pgx/v5 (github.com/jackc/pgx/v5)
- **Database**: PostgreSQL 16

## Database Connection

```go
import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Connection string format
connStr := "postgres://user:password@host:5432/nom_indexer?sslmode=disable"

pool, err := pgxpool.New(context.Background(), connStr)
```

## Database Schema

### Tables Overview

| Table | Primary Key | Description |
|-------|-------------|-------------|
| `momentums` | `height` | Block headers |
| `accounts` | `address` | User accounts |
| `account_blocks` | `hash` | Transactions |
| `balances` | `(address, token_standard)` | Token balances |
| `tokens` | `token_standard` | ZTS token registry |
| `pillars` | `owner_address` | Validator nodes |
| `pillar_updates` | `id` (serial) | Pillar config history |
| `sentinels` | `owner` | Sentinel nodes |
| `stakes` | `id` | Staking entries |
| `fusions` | `id` | Plasma fusion entries |
| `projects` | `id` | Accelerator-Z projects |
| `project_phases` | `id` | Project phases |
| `votes` | `id` (serial) | Pillar votes |
| `cumulative_rewards` | `id` (serial) | Aggregated rewards |
| `reward_transactions` | `hash` | Individual reward TXs |
| `wrap_token_requests` | `id` | Bridge wrap operations |
| `unwrap_token_requests` | `(transaction_hash, log_index)` | Bridge unwrap operations |

---

## Data Models

### Momentum
```go
type Momentum struct {
    Height        uint64 `json:"height"`
    Hash          string `json:"hash"`
    Timestamp     int64  `json:"timestamp"`
    TxCount       int    `json:"txCount"`
    Producer      string `json:"producer"`
    ProducerOwner string `json:"producerOwner"`
    ProducerName  string `json:"producerName"`
}
```

**SQL Schema:**
```sql
CREATE TABLE momentums (
    height BIGINT PRIMARY KEY,
    hash TEXT NOT NULL,
    timestamp BIGINT NOT NULL,
    tx_count INT NOT NULL,
    producer TEXT NOT NULL,
    producer_owner TEXT NOT NULL DEFAULT '',
    producer_name TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_momentums_timestamp ON momentums(timestamp);
CREATE INDEX idx_momentums_producer ON momentums(producer);
```

### Account
```go
type Account struct {
    Address                  string `json:"address"`
    BlockCount               int64  `json:"blockCount"`
    PublicKey                string `json:"publicKey"`
    Delegate                 string `json:"delegate"`
    DelegationStartTimestamp int64  `json:"delegationStartTimestamp"`
}
```

**SQL Schema:**
```sql
CREATE TABLE accounts (
    address TEXT PRIMARY KEY,
    block_count BIGINT NOT NULL,
    public_key TEXT,
    delegate TEXT NOT NULL DEFAULT '',
    delegation_start_timestamp BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_accounts_delegate ON accounts(delegate);
```

### AccountBlock (Transaction)
```go
type AccountBlock struct {
    Hash               string          `json:"hash"`
    MomentumHash       string          `json:"momentumHash"`
    MomentumTimestamp  int64           `json:"momentumTimestamp"`
    MomentumHeight     int64           `json:"momentumHeight"`
    BlockType          int16           `json:"blockType"`
    Height             int64           `json:"height"`
    Address            string          `json:"address"`
    ToAddress          string          `json:"toAddress"`
    Amount             int64           `json:"amount"`
    TokenStandard      string          `json:"tokenStandard"`
    Data               string          `json:"data"`
    Method             string          `json:"method"`
    Input              json.RawMessage `json:"input"`
    PairedAccountBlock string          `json:"pairedAccountBlock"`
    DescendantOf       string          `json:"descendantOf"`
}
```

**Block Types:**
| Value | Type |
|-------|------|
| 1 | Genesis Receive |
| 2 | User Send |
| 3 | User Receive |
| 4 | Contract Send |
| 5 | Contract Receive |

**SQL Schema:**
```sql
CREATE TABLE account_blocks (
    hash TEXT PRIMARY KEY,
    momentum_hash TEXT,
    momentum_timestamp BIGINT,
    momentum_height BIGINT,
    block_type SMALLINT NOT NULL,
    height BIGINT NOT NULL,
    address TEXT NOT NULL,
    to_address TEXT,
    amount BIGINT NOT NULL,
    token_standard TEXT,
    data TEXT,
    method TEXT DEFAULT '',
    input JSONB DEFAULT '{}',
    paired_account_block TEXT DEFAULT '',
    descendant_of TEXT DEFAULT ''
);

CREATE INDEX idx_account_blocks_address ON account_blocks(address);
CREATE INDEX idx_account_blocks_to_address ON account_blocks(to_address);
CREATE INDEX idx_account_blocks_momentum_height ON account_blocks(momentum_height);
CREATE INDEX idx_account_blocks_token_standard ON account_blocks(token_standard);
CREATE INDEX idx_account_blocks_method ON account_blocks(method);
```

### Balance
```go
type Balance struct {
    Address       string `json:"address"`
    TokenStandard string `json:"tokenStandard"`
    Balance       int64  `json:"balance"`
}
```

**SQL Schema:**
```sql
CREATE TABLE balances (
    address TEXT NOT NULL,
    token_standard TEXT NOT NULL,
    balance BIGINT NOT NULL,
    PRIMARY KEY (address, token_standard)
);

CREATE INDEX idx_balances_token_standard ON balances(token_standard);
CREATE INDEX idx_balances_balance ON balances(balance) WHERE balance > 0;
```

### Token
```go
type Token struct {
    TokenStandard       string `json:"tokenStandard"`
    Name                string `json:"name"`
    Symbol              string `json:"symbol"`
    Domain              string `json:"domain"`
    Decimals            int    `json:"decimals"`
    Owner               string `json:"owner"`
    TotalSupply         int64  `json:"totalSupply"`
    MaxSupply           int64  `json:"maxSupply"`
    IsBurnable          bool   `json:"isBurnable"`
    IsMintable          bool   `json:"isMintable"`
    IsUtility           bool   `json:"isUtility"`
    TotalBurned         int64  `json:"totalBurned"`
    LastUpdateTimestamp int64  `json:"lastUpdateTimestamp"`
    HolderCount         int64  `json:"holderCount"`
    TransactionCount    int64  `json:"transactionCount"`
}
```

**SQL Schema:**
```sql
CREATE TABLE tokens (
    token_standard TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    domain TEXT,
    decimals INT NOT NULL,
    owner TEXT NOT NULL,
    total_supply BIGINT NOT NULL,
    max_supply BIGINT NOT NULL,
    is_burnable BOOLEAN NOT NULL,
    is_mintable BOOLEAN NOT NULL,
    is_utility BOOLEAN NOT NULL,
    total_burned BIGINT NOT NULL DEFAULT 0,
    last_update_timestamp BIGINT NOT NULL DEFAULT 0,
    holder_count BIGINT NOT NULL DEFAULT 0,
    transaction_count BIGINT NOT NULL DEFAULT 0
);
```

### Pillar
```go
type Pillar struct {
    OwnerAddress                 string  `json:"ownerAddress"`
    ProducerAddress              string  `json:"producerAddress"`
    WithdrawAddress              string  `json:"withdrawAddress"`
    Name                         string  `json:"name"`
    Rank                         int     `json:"rank"`
    GiveMomentumRewardPercentage int16   `json:"giveMomentumRewardPercentage"`
    GiveDelegateRewardPercentage int16   `json:"giveDelegateRewardPercentage"`
    IsRevocable                  bool    `json:"isRevocable"`
    RevokeCooldown               int     `json:"revokeCooldown"`
    RevokeTimestamp              int64   `json:"revokeTimestamp"`
    Weight                       int64   `json:"weight"`
    EpochProducedMomentums       int16   `json:"epochProducedMomentums"`
    EpochExpectedMomentums       int16   `json:"epochExpectedMomentums"`
    SlotCostQsr                  int64   `json:"slotCostQsr"`
    SpawnTimestamp               int64   `json:"spawnTimestamp"`
    VotingActivity               float32 `json:"votingActivity"`
    ProducedMomentumCount        int64   `json:"producedMomentumCount"`
    IsRevoked                    bool    `json:"isRevoked"`
}
```

**SQL Schema:**
```sql
CREATE TABLE pillars (
    owner_address TEXT PRIMARY KEY,
    producer_address TEXT NOT NULL,
    withdraw_address TEXT NOT NULL,
    name TEXT NOT NULL,
    rank INT NOT NULL,
    give_momentum_reward_percentage SMALLINT NOT NULL,
    give_delegate_reward_percentage SMALLINT NOT NULL,
    is_revocable BOOLEAN NOT NULL,
    revoke_cooldown INT NOT NULL,
    revoke_timestamp BIGINT NOT NULL,
    weight BIGINT NOT NULL,
    epoch_produced_momentums SMALLINT NOT NULL,
    epoch_expected_momentums SMALLINT NOT NULL,
    slot_cost_qsr BIGINT NOT NULL DEFAULT 0,
    spawn_timestamp BIGINT NOT NULL DEFAULT 0,
    voting_activity REAL NOT NULL DEFAULT 0,
    produced_momentum_count BIGINT NOT NULL DEFAULT 0,
    is_revoked BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_pillars_name ON pillars(name);
CREATE INDEX idx_pillars_producer_address ON pillars(producer_address);
CREATE INDEX idx_pillars_withdraw_address ON pillars(withdraw_address);
```

### PillarUpdate
```go
type PillarUpdate struct {
    ID                           int    `json:"id"`
    Name                         string `json:"name"`
    OwnerAddress                 string `json:"ownerAddress"`
    ProducerAddress              string `json:"producerAddress"`
    WithdrawAddress              string `json:"withdrawAddress"`
    MomentumTimestamp            int64  `json:"momentumTimestamp"`
    MomentumHeight               int64  `json:"momentumHeight"`
    MomentumHash                 string `json:"momentumHash"`
    GiveMomentumRewardPercentage int16  `json:"giveMomentumRewardPercentage"`
    GiveDelegateRewardPercentage int16  `json:"giveDelegateRewardPercentage"`
}
```

**SQL Schema:**
```sql
CREATE TABLE pillar_updates (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    owner_address TEXT NOT NULL,
    producer_address TEXT NOT NULL,
    withdraw_address TEXT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    momentum_height BIGINT NOT NULL,
    momentum_hash TEXT NOT NULL,
    give_momentum_reward_percentage SMALLINT NOT NULL,
    give_delegate_reward_percentage SMALLINT NOT NULL
);

CREATE INDEX idx_pillar_updates_owner_address ON pillar_updates(owner_address);
CREATE INDEX idx_pillar_updates_momentum_height ON pillar_updates(momentum_height);
```

### Sentinel
```go
type Sentinel struct {
    Owner                 string `json:"owner"`
    RegistrationTimestamp int64  `json:"registrationTimestamp"`
    IsRevocable           bool   `json:"isRevocable"`
    RevokeCooldown        string `json:"revokeCooldown"`
    Active                bool   `json:"active"`
}
```

**SQL Schema:**
```sql
CREATE TABLE sentinels (
    owner TEXT PRIMARY KEY,
    registration_timestamp BIGINT NOT NULL,
    is_revocable BOOLEAN NOT NULL,
    revoke_cooldown TEXT NOT NULL,
    active BOOLEAN NOT NULL
);
```

### Stake
```go
type Stake struct {
    ID                  string `json:"id"`
    Address             string `json:"address"`
    StartTimestamp      int64  `json:"startTimestamp"`
    ExpirationTimestamp int64  `json:"expirationTimestamp"`
    ZnnAmount           int64  `json:"znnAmount"`
    DurationInSec       int    `json:"durationInSec"`
    IsActive            bool   `json:"isActive"`
    CancelID            string `json:"cancelId"`
}
```

**SQL Schema:**
```sql
CREATE TABLE stakes (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    start_timestamp BIGINT NOT NULL,
    expiration_timestamp BIGINT NOT NULL,
    znn_amount BIGINT NOT NULL,
    duration_in_sec INT NOT NULL,
    is_active BOOLEAN NOT NULL,
    cancel_id TEXT NOT NULL
);

CREATE INDEX idx_stakes_address ON stakes(address);
CREATE INDEX idx_stakes_is_active ON stakes(is_active);
```

### Fusion
```go
type Fusion struct {
    ID                string `json:"id"`
    Address           string `json:"address"`
    Beneficiary       string `json:"beneficiary"`
    MomentumHash      string `json:"momentumHash"`
    MomentumTimestamp int64  `json:"momentumTimestamp"`
    MomentumHeight    int64  `json:"momentumHeight"`
    QsrAmount         int64  `json:"qsrAmount"`
    ExpirationHeight  int64  `json:"expirationHeight"`
    IsActive          bool   `json:"isActive"`
    CancelID          string `json:"cancelId"`
}
```

**SQL Schema:**
```sql
CREATE TABLE fusions (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    beneficiary TEXT NOT NULL,
    momentum_hash TEXT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    momentum_height BIGINT NOT NULL,
    qsr_amount BIGINT NOT NULL,
    expiration_height BIGINT NOT NULL,
    is_active BOOLEAN NOT NULL,
    cancel_id TEXT NOT NULL
);

CREATE INDEX idx_fusions_address ON fusions(address);
CREATE INDEX idx_fusions_beneficiary ON fusions(beneficiary);
CREATE INDEX idx_fusions_is_active ON fusions(is_active);
```

### Project (Accelerator-Z)
```go
type Project struct {
    ID                  string `json:"id"`
    VotingID            string `json:"votingId"`
    Owner               string `json:"owner"`
    Name                string `json:"name"`
    Description         string `json:"description"`
    URL                 string `json:"url"`
    ZnnFundsNeeded      int64  `json:"znnFundsNeeded"`
    QsrFundsNeeded      int64  `json:"qsrFundsNeeded"`
    CreationTimestamp   int64  `json:"creationTimestamp"`
    LastUpdateTimestamp int64  `json:"lastUpdateTimestamp"`
    Status              int16  `json:"status"`
    YesVotes            int16  `json:"yesVotes"`
    NoVotes             int16  `json:"noVotes"`
    TotalVotes          int16  `json:"totalVotes"`
}
```

**Project Status Values:**
| Value | Status |
|-------|--------|
| 0 | Voting |
| 1 | Active |
| 2 | Paid |
| 3 | Closed |
| 4 | Completed |

**SQL Schema:**
```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    voting_id TEXT NOT NULL,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    url TEXT,
    znn_funds_needed BIGINT NOT NULL,
    qsr_funds_needed BIGINT NOT NULL,
    creation_timestamp BIGINT NOT NULL,
    last_update_timestamp BIGINT NOT NULL,
    status SMALLINT NOT NULL,
    yes_votes SMALLINT NOT NULL DEFAULT 0,
    no_votes SMALLINT NOT NULL DEFAULT 0,
    total_votes SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_projects_owner ON projects(owner);
CREATE INDEX idx_projects_voting_id ON projects(voting_id);
```

### ProjectPhase
```go
type ProjectPhase struct {
    ID                string `json:"id"`
    ProjectID         string `json:"projectId"`
    VotingID          string `json:"votingId"`
    Name              string `json:"name"`
    Description       string `json:"description"`
    URL               string `json:"url"`
    ZnnFundsNeeded    int64  `json:"znnFundsNeeded"`
    QsrFundsNeeded    int64  `json:"qsrFundsNeeded"`
    CreationTimestamp int64  `json:"creationTimestamp"`
    AcceptedTimestamp int64  `json:"acceptedTimestamp"`
    Status            int16  `json:"status"`
    YesVotes          int16  `json:"yesVotes"`
    NoVotes           int16  `json:"noVotes"`
    TotalVotes        int16  `json:"totalVotes"`
}
```

**SQL Schema:**
```sql
CREATE TABLE project_phases (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    voting_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    url TEXT,
    znn_funds_needed BIGINT NOT NULL,
    qsr_funds_needed BIGINT NOT NULL,
    creation_timestamp BIGINT NOT NULL,
    accepted_timestamp BIGINT NOT NULL,
    status SMALLINT NOT NULL,
    yes_votes SMALLINT NOT NULL DEFAULT 0,
    no_votes SMALLINT NOT NULL DEFAULT 0,
    total_votes SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_project_phases_project_id ON project_phases(project_id);
CREATE INDEX idx_project_phases_voting_id ON project_phases(voting_id);
```

### Vote
```go
type Vote struct {
    ID                int    `json:"id"`
    MomentumHash      string `json:"momentumHash"`
    MomentumTimestamp int64  `json:"momentumTimestamp"`
    MomentumHeight    int64  `json:"momentumHeight"`
    VoterAddress      string `json:"voterAddress"`
    ProjectID         string `json:"projectId"`
    PhaseID           string `json:"phaseId"`
    VotingID          string `json:"votingId"`
    Vote              int16  `json:"vote"`
}
```

**Vote Values:**
| Value | Meaning |
|-------|---------|
| 0 | No |
| 1 | Yes |
| 2 | Abstain |

**SQL Schema:**
```sql
CREATE TABLE votes (
    id SERIAL PRIMARY KEY,
    momentum_hash TEXT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    momentum_height BIGINT NOT NULL,
    voter_address TEXT NOT NULL,
    project_id TEXT NOT NULL,
    phase_id TEXT NOT NULL DEFAULT '',
    voting_id TEXT NOT NULL,
    vote SMALLINT NOT NULL
);

CREATE INDEX idx_votes_voter_address ON votes(voter_address);
CREATE INDEX idx_votes_project_id ON votes(project_id);
CREATE INDEX idx_votes_phase_id ON votes(phase_id);
CREATE INDEX idx_votes_voting_id ON votes(voting_id);
```

### RewardTransaction
```go
type RewardTransaction struct {
    Hash              string `json:"hash"`
    Address           string `json:"address"`
    RewardType        int    `json:"rewardType"`
    MomentumTimestamp int64  `json:"momentumTimestamp"`
    MomentumHeight    int64  `json:"momentumHeight"`
    AccountHeight     int64  `json:"accountHeight"`
    Amount            int64  `json:"amount"`
    TokenStandard     string `json:"tokenStandard"`
    SourceAddress     string `json:"sourceAddress"`
}
```

**Reward Types:**
| Value | Type |
|-------|------|
| 0 | Unknown |
| 1 | Stake |
| 2 | Delegation |
| 3 | Liquidity |
| 4 | Sentinel |
| 5 | Pillar |

**SQL Schema:**
```sql
CREATE TABLE reward_transactions (
    hash TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    reward_type SMALLINT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    momentum_height BIGINT NOT NULL,
    account_height BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    token_standard TEXT NOT NULL,
    source_address TEXT NOT NULL
);

CREATE INDEX idx_reward_transactions_address ON reward_transactions(address);
CREATE INDEX idx_reward_transactions_reward_type ON reward_transactions(reward_type);
CREATE INDEX idx_reward_transactions_momentum_height ON reward_transactions(momentum_height);
```

### CumulativeReward
```go
type CumulativeReward struct {
    ID            int    `json:"id"`
    Address       string `json:"address"`
    RewardType    int    `json:"rewardType"`
    Amount        int64  `json:"amount"`
    TokenStandard string `json:"tokenStandard"`
}
```

**SQL Schema:**
```sql
CREATE TABLE cumulative_rewards (
    id SERIAL PRIMARY KEY,
    address TEXT NOT NULL,
    reward_type SMALLINT NOT NULL,
    amount BIGINT NOT NULL,
    token_standard TEXT NOT NULL,
    UNIQUE (address, reward_type, token_standard)
);
```

### WrapTokenRequest (Bridge)
```go
type WrapTokenRequest struct {
    ID                      string `json:"id"`
    NetworkClass            int    `json:"networkClass"`
    ChainID                 int    `json:"chainId"`
    ToAddress               string `json:"toAddress"`
    TokenStandard           string `json:"tokenStandard"`
    TokenAddress            string `json:"tokenAddress"`
    Amount                  int64  `json:"amount"`
    Fee                     int64  `json:"fee"`
    Signature               string `json:"signature"`
    CreationMomentumHeight  int64  `json:"creationMomentumHeight"`
    ConfirmationsToFinality int    `json:"confirmationsToFinality"`
}
```

**SQL Schema:**
```sql
CREATE TABLE wrap_token_requests (
    id TEXT PRIMARY KEY,
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    to_address TEXT NOT NULL,
    token_standard TEXT NOT NULL,
    token_address TEXT NOT NULL,
    amount BIGINT NOT NULL,
    fee BIGINT NOT NULL,
    signature TEXT NOT NULL,
    creation_momentum_height BIGINT NOT NULL,
    confirmations_to_finality INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_wrap_token_requests_to_address ON wrap_token_requests(to_address);
CREATE INDEX idx_wrap_token_requests_token_standard ON wrap_token_requests(token_standard);
CREATE INDEX idx_wrap_token_requests_chain ON wrap_token_requests(network_class, chain_id);
```

### UnwrapTokenRequest (Bridge)
```go
type UnwrapTokenRequest struct {
    TransactionHash            string `json:"transactionHash"`
    LogIndex                   int64  `json:"logIndex"`
    NetworkClass               int    `json:"networkClass"`
    ChainID                    int    `json:"chainId"`
    ToAddress                  string `json:"toAddress"`
    TokenStandard              string `json:"tokenStandard"`
    TokenAddress               string `json:"tokenAddress"`
    Amount                     int64  `json:"amount"`
    Signature                  string `json:"signature"`
    RegistrationMomentumHeight int64  `json:"registrationMomentumHeight"`
    Redeemed                   bool   `json:"redeemed"`
    Revoked                    bool   `json:"revoked"`
    RedeemableIn               int64  `json:"redeemableIn"`
}
```

**SQL Schema:**
```sql
CREATE TABLE unwrap_token_requests (
    transaction_hash TEXT NOT NULL,
    log_index BIGINT NOT NULL,
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    to_address TEXT NOT NULL,
    token_standard TEXT NOT NULL,
    token_address TEXT NOT NULL,
    amount BIGINT NOT NULL,
    signature TEXT NOT NULL,
    registration_momentum_height BIGINT NOT NULL,
    redeemed BOOLEAN NOT NULL DEFAULT FALSE,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    redeemable_in BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (transaction_hash, log_index)
);

CREATE INDEX idx_unwrap_token_requests_to_address ON unwrap_token_requests(to_address);
CREATE INDEX idx_unwrap_token_requests_status ON unwrap_token_requests(redeemed, revoked);
```

---

## Constants

### Embedded Contract Addresses
```go
const (
    PlasmaAddress      = "z1qxemdeddedxplasmaxxxxxxxxxxxxxxxxsctrp"
    PillarAddress      = "z1qxemdeddedxpyllarxxxxxxxxxxxxxxxsy3fmg"
    TokenAddress       = "z1qxemdeddedxt0kenxxxxxxxxxxxxxxxxh9amk0"
    SentinelAddress    = "z1qxemdeddedxsentynelxxxxxxxxxxxxxwy0r2r"
    StakeAddress       = "z1qxemdeddedxstakexxxxxxxxxxxxxxxxjv8v62"
    AcceleratorAddress = "z1qxemdeddedxaccelerat0rxxxxxxxxxxp4tk22"
    SwapAddress        = "z1qxemdeddedxswapxxxxxxxxxxxxxxxxxxl4yww"
    LiquidityAddress   = "z1qxemdeddedxlyquydytyxxxxxxxxxxxxflaaae"
    BridgeAddress      = "z1qxemdeddedxdrydgexxxxxxxxxxxxxxxmqgr0d"
    HtlcAddress        = "z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw"
    SporkAddress       = "z1qxemdeddedxsp0rkxxxxxxxxxxxxxxxx956u48"
)
```

### Special Addresses
```go
const (
    EmptyAddress             = "z1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsggv2f"
    LiquidityTreasuryAddress = "z1qqw8f3qxx9zg92xgckqdpfws3dw07d26afsj74"
)
```

### Token Standards
```go
const (
    EmptyTokenStandard = "zts1qqqqqqqqqqqqqqqqtq587y"
    ZnnTokenStandard   = "zts1znnxxxxxxxxxxxxx9z4ulx"
    QsrTokenStandard   = "zts1qsrxxxxxxxxxxxxxmrhjll"
)
```

---

## Suggested API Endpoints

### Blockchain
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/status` | Indexer status (latest height, sync status) |
| GET | `/api/v1/momentums` | List momentums (paginated) |
| GET | `/api/v1/momentums/:height` | Get momentum by height |
| GET | `/api/v1/momentums/latest` | Get latest momentum |

### Transactions
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/transactions` | List transactions (paginated, filterable) |
| GET | `/api/v1/transactions/:hash` | Get transaction by hash |
| GET | `/api/v1/accounts/:address/transactions` | Transactions for an address |

### Accounts & Balances
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/accounts/:address` | Get account info |
| GET | `/api/v1/accounts/:address/balances` | Get all balances for address |
| GET | `/api/v1/balances` | List all balances (filterable by token) |

### Tokens
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tokens` | List all tokens |
| GET | `/api/v1/tokens/:tokenStandard` | Get token by standard |
| GET | `/api/v1/tokens/:tokenStandard/holders` | List token holders |

### Pillars
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/pillars` | List all active pillars |
| GET | `/api/v1/pillars/:name` | Get pillar by name |
| GET | `/api/v1/pillars/:name/delegators` | List delegators for pillar |
| GET | `/api/v1/pillars/:name/history` | Pillar configuration history |

### Sentinels
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/sentinels` | List all sentinels |
| GET | `/api/v1/sentinels/:owner` | Get sentinel by owner |

### Staking
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/stakes` | List all active stakes |
| GET | `/api/v1/accounts/:address/stakes` | Stakes for an address |

### Plasma (Fusions)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/fusions` | List all active fusions |
| GET | `/api/v1/accounts/:address/fusions` | Fusions for an address |

### Accelerator-Z
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/projects` | List all projects |
| GET | `/api/v1/projects/:id` | Get project by ID |
| GET | `/api/v1/projects/:id/phases` | Get phases for project |
| GET | `/api/v1/projects/:id/votes` | Get votes for project |

### Rewards
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/accounts/:address/rewards` | Cumulative rewards for address |
| GET | `/api/v1/accounts/:address/rewards/history` | Reward transaction history |

### Bridge
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/bridge/wraps` | List wrap requests |
| GET | `/api/v1/bridge/unwraps` | List unwrap requests |
| GET | `/api/v1/accounts/:address/bridge/wraps` | Wraps for address |
| GET | `/api/v1/accounts/:address/bridge/unwraps` | Unwraps for address |

---

## Common Query Patterns

### Pagination
```go
// Standard pagination query
SELECT * FROM table_name
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
```

### Get Latest Momentum Height
```sql
SELECT MAX(height) FROM momentums
```

### Get Transactions by Address
```sql
SELECT * FROM account_blocks
WHERE address = $1 OR to_address = $1
ORDER BY momentum_height DESC, height DESC
LIMIT $2 OFFSET $3
```

### Get Token Holders
```sql
SELECT address, balance FROM balances
WHERE token_standard = $1 AND balance > 0
ORDER BY balance DESC
LIMIT $2 OFFSET $3
```

### Get Active Pillars
```sql
SELECT * FROM pillars
WHERE is_revoked = false
ORDER BY rank ASC
```

### Get Delegators for a Pillar
```sql
SELECT a.address, b.balance as znn_balance
FROM accounts a
JOIN balances b ON a.address = b.address
WHERE a.delegate = $1
  AND b.token_standard = 'zts1znnxxxxxxxxxxxxx9z4ulx'
  AND b.balance > 0
ORDER BY b.balance DESC
```

### Get Cumulative Rewards by Address
```sql
SELECT reward_type, token_standard, SUM(amount) as total
FROM reward_transactions
WHERE address = $1
GROUP BY reward_type, token_standard
```

---

## Example Gin Router Setup

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
)

func SetupRouter(pool *pgxpool.Pool) *gin.Engine {
    r := gin.Default()

    // Middleware
    r.Use(gin.Recovery())
    r.Use(CORSMiddleware())

    v1 := r.Group("/api/v1")
    {
        // Status
        v1.GET("/status", getStatus(pool))

        // Momentums
        v1.GET("/momentums", listMomentums(pool))
        v1.GET("/momentums/latest", getLatestMomentum(pool))
        v1.GET("/momentums/:height", getMomentum(pool))

        // Transactions
        v1.GET("/transactions", listTransactions(pool))
        v1.GET("/transactions/:hash", getTransaction(pool))

        // Accounts
        v1.GET("/accounts/:address", getAccount(pool))
        v1.GET("/accounts/:address/balances", getAccountBalances(pool))
        v1.GET("/accounts/:address/transactions", getAccountTransactions(pool))
        v1.GET("/accounts/:address/stakes", getAccountStakes(pool))
        v1.GET("/accounts/:address/fusions", getAccountFusions(pool))
        v1.GET("/accounts/:address/rewards", getAccountRewards(pool))

        // Tokens
        v1.GET("/tokens", listTokens(pool))
        v1.GET("/tokens/:tokenStandard", getToken(pool))
        v1.GET("/tokens/:tokenStandard/holders", getTokenHolders(pool))

        // Pillars
        v1.GET("/pillars", listPillars(pool))
        v1.GET("/pillars/:name", getPillar(pool))
        v1.GET("/pillars/:name/delegators", getPillarDelegators(pool))

        // Sentinels
        v1.GET("/sentinels", listSentinels(pool))

        // Projects
        v1.GET("/projects", listProjects(pool))
        v1.GET("/projects/:id", getProject(pool))
        v1.GET("/projects/:id/phases", getProjectPhases(pool))

        // Bridge
        v1.GET("/bridge/wraps", listWraps(pool))
        v1.GET("/bridge/unwraps", listUnwraps(pool))
    }

    return r
}
```

---

## Response Format

### Success Response
```json
{
    "data": { ... },
    "meta": {
        "page": 1,
        "pageSize": 20,
        "total": 1000
    }
}
```

### Error Response
```json
{
    "error": {
        "code": "NOT_FOUND",
        "message": "Resource not found"
    }
}
```

---

## Notes

1. **All amounts are in the smallest unit** (e.g., 1 ZNN = 100000000 units, 8 decimals)
2. **Timestamps are Unix timestamps** (seconds since epoch)
3. **The database is read-only** for the API - the indexer handles all writes
4. **Consider adding caching** (Redis) for frequently accessed data like latest momentum, pillars list
5. **Add rate limiting** to protect against abuse
6. **Token standards** follow the format `zts1...` (Zenon Token Standard)
7. **Addresses** follow the format `z1...` (Zenon address format)
