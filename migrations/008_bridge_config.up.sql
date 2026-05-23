-- Bridge configuration mirrored from the BridgeApi: networks, per-network
-- token pairs, admins, guardians, and singleton orchestrator + security info.

CREATE TABLE IF NOT EXISTS bridge_networks (
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    name TEXT NOT NULL,
    contract_address TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '',
    last_updated_timestamp BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (network_class, chain_id)
);

CREATE TABLE IF NOT EXISTS bridge_network_tokens (
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    token_standard TEXT NOT NULL,
    token_address TEXT NOT NULL,
    bridgeable BOOLEAN NOT NULL DEFAULT false,
    redeemable BOOLEAN NOT NULL DEFAULT false,
    owned BOOLEAN NOT NULL DEFAULT false,
    min_amount BIGINT NOT NULL DEFAULT 0,
    fee_percentage INT NOT NULL DEFAULT 0,
    redeem_delay INT NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (network_class, chain_id, token_standard)
);

-- The administrator and guardians are point-in-time pulled from the SecurityInfo
-- and BridgeInfo. The administrator is a singleton; we only need the current
-- value, so we store with a fixed row_id = 1.
CREATE TABLE IF NOT EXISTS bridge_admin (
    row_id SMALLINT PRIMARY KEY CHECK (row_id = 1),
    administrator TEXT NOT NULL,
    compressed_tss_ecdsa_pubkey TEXT NOT NULL DEFAULT '',
    decompressed_tss_ecdsa_pubkey TEXT NOT NULL DEFAULT '',
    allow_key_gen BOOLEAN NOT NULL DEFAULT false,
    halted BOOLEAN NOT NULL DEFAULT false,
    unhalted_at BIGINT NOT NULL DEFAULT 0,
    unhalt_duration_in_momentums BIGINT NOT NULL DEFAULT 0,
    tss_nonce BIGINT NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '',
    last_updated_timestamp BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS bridge_guardians (
    address TEXT PRIMARY KEY,
    nominated BOOLEAN NOT NULL DEFAULT false,
    accepted BOOLEAN NOT NULL DEFAULT false,
    last_updated_timestamp BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS bridge_orchestrator_info (
    row_id SMALLINT PRIMARY KEY CHECK (row_id = 1),
    window_size BIGINT NOT NULL DEFAULT 0,
    key_gen_threshold INT NOT NULL DEFAULT 0,
    confirmations_to_finality INT NOT NULL DEFAULT 0,
    estimated_momentum_time INT NOT NULL DEFAULT 0,
    allow_key_gen_height BIGINT NOT NULL DEFAULT 0,
    last_updated_timestamp BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS bridge_security_info (
    row_id SMALLINT PRIMARY KEY CHECK (row_id = 1),
    administrator_delay BIGINT NOT NULL DEFAULT 0,
    soft_delay BIGINT NOT NULL DEFAULT 0,
    last_updated_timestamp BIGINT NOT NULL DEFAULT 0
);
