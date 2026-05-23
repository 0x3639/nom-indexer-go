-- Daily snapshot tables for network-wide, per-token, per-pillar, and per-bridge
-- metrics. Each row's natural key is (date, foreign_key); the cron loop upserts
-- the current day's row on each tick.

CREATE TABLE IF NOT EXISTS network_stat_histories (
    date DATE PRIMARY KEY,
    total_tx BIGINT NOT NULL DEFAULT 0,
    daily_tx BIGINT NOT NULL DEFAULT 0,
    total_addresses BIGINT NOT NULL DEFAULT 0,
    daily_addresses BIGINT NOT NULL DEFAULT 0,
    active_addresses BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    daily_tokens BIGINT NOT NULL DEFAULT 0,
    total_stakes BIGINT NOT NULL DEFAULT 0,
    daily_stakes BIGINT NOT NULL DEFAULT 0,
    total_fusions BIGINT NOT NULL DEFAULT 0,
    daily_fusions BIGINT NOT NULL DEFAULT 0,
    total_pillars BIGINT NOT NULL DEFAULT 0,
    total_sentinels BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS token_stat_histories (
    date DATE NOT NULL,
    token_standard TEXT NOT NULL,
    daily_minted BIGINT NOT NULL DEFAULT 0,
    daily_burned BIGINT NOT NULL DEFAULT 0,
    total_supply BIGINT NOT NULL DEFAULT 0,
    total_holders BIGINT NOT NULL DEFAULT 0,
    total_transactions BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date, token_standard)
);

CREATE INDEX IF NOT EXISTS idx_token_stat_histories_token ON token_stat_histories(token_standard);

CREATE TABLE IF NOT EXISTS pillar_stat_histories (
    date DATE NOT NULL,
    pillar_owner_address TEXT NOT NULL,
    rank INT NOT NULL DEFAULT 0,
    weight BIGINT NOT NULL DEFAULT 0,
    momentum_rewards BIGINT NOT NULL DEFAULT 0,
    delegate_rewards BIGINT NOT NULL DEFAULT 0,
    total_delegators BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date, pillar_owner_address)
);

CREATE INDEX IF NOT EXISTS idx_pillar_stat_histories_owner ON pillar_stat_histories(pillar_owner_address);

CREATE TABLE IF NOT EXISTS bridge_stat_histories (
    date DATE NOT NULL,
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    token_standard TEXT NOT NULL,
    wrap_tx_count BIGINT NOT NULL DEFAULT 0,
    wrapped_amount BIGINT NOT NULL DEFAULT 0,
    unwrap_tx_count BIGINT NOT NULL DEFAULT 0,
    unwrapped_amount BIGINT NOT NULL DEFAULT 0,
    total_volume BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (date, network_class, chain_id, token_standard)
);
