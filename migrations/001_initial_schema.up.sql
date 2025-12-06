-- Momentums table (blockchain blocks)
CREATE TABLE IF NOT EXISTS momentums (
    height BIGINT PRIMARY KEY,
    hash TEXT NOT NULL,
    timestamp BIGINT NOT NULL,
    tx_count INT NOT NULL,
    producer TEXT NOT NULL,
    producer_owner TEXT NOT NULL DEFAULT '',
    producer_name TEXT NOT NULL DEFAULT ''
);

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    address TEXT PRIMARY KEY,
    block_count BIGINT NOT NULL,
    public_key TEXT,
    delegate TEXT NOT NULL DEFAULT '',
    delegation_start_timestamp BIGINT NOT NULL DEFAULT 0
);

-- Balances table
CREATE TABLE IF NOT EXISTS balances (
    address TEXT NOT NULL,
    token_standard TEXT NOT NULL,
    balance BIGINT NOT NULL,
    PRIMARY KEY (address, token_standard)
);

-- Account blocks table (transactions)
CREATE TABLE IF NOT EXISTS account_blocks (
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

-- Tokens table
CREATE TABLE IF NOT EXISTS tokens (
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

-- Pillars table (validators)
CREATE TABLE IF NOT EXISTS pillars (
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

-- Pillar updates table (history of pillar changes)
CREATE TABLE IF NOT EXISTS pillar_updates (
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

-- Sentinels table
CREATE TABLE IF NOT EXISTS sentinels (
    owner TEXT PRIMARY KEY,
    registration_timestamp BIGINT NOT NULL,
    is_revocable BOOLEAN NOT NULL,
    revoke_cooldown TEXT NOT NULL,
    active BOOLEAN NOT NULL
);

-- Stakes table
CREATE TABLE IF NOT EXISTS stakes (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    start_timestamp BIGINT NOT NULL,
    expiration_timestamp BIGINT NOT NULL,
    znn_amount BIGINT NOT NULL,
    duration_in_sec INT NOT NULL,
    is_active BOOLEAN NOT NULL,
    cancel_id TEXT NOT NULL
);

-- Projects table (Accelerator-Z)
CREATE TABLE IF NOT EXISTS projects (
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

-- Project phases table
CREATE TABLE IF NOT EXISTS project_phases (
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

-- Votes table
CREATE TABLE IF NOT EXISTS votes (
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

-- Fusions table (plasma)
CREATE TABLE IF NOT EXISTS fusions (
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

-- Cumulative rewards table
CREATE TABLE IF NOT EXISTS cumulative_rewards (
    id SERIAL PRIMARY KEY,
    address TEXT NOT NULL,
    reward_type SMALLINT NOT NULL,
    amount BIGINT NOT NULL,
    token_standard TEXT NOT NULL,
    UNIQUE (address, reward_type, token_standard)
);

-- Reward transactions table
CREATE TABLE IF NOT EXISTS reward_transactions (
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
