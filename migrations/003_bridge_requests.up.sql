-- Bridge wrap token requests (Zenon -> External chain)
CREATE TABLE IF NOT EXISTS wrap_token_requests (
    id TEXT PRIMARY KEY,
    network_class INT NOT NULL,
    chain_id INT NOT NULL,
    to_address TEXT NOT NULL,
    token_standard TEXT NOT NULL,
    token_address TEXT NOT NULL,
    amount BIGINT NOT NULL,
    fee BIGINT NOT NULL,
    signature TEXT NOT NULL,
    creation_momentum_height BIGINT NOT NULL
);

-- Bridge unwrap token requests (External chain -> Zenon)
CREATE TABLE IF NOT EXISTS unwrap_token_requests (
    transaction_hash TEXT NOT NULL,
    log_index INT NOT NULL,
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
    PRIMARY KEY (transaction_hash, log_index)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_wrap_token_requests_to_address ON wrap_token_requests(to_address);
CREATE INDEX IF NOT EXISTS idx_wrap_token_requests_token_standard ON wrap_token_requests(token_standard);
CREATE INDEX IF NOT EXISTS idx_wrap_token_requests_chain ON wrap_token_requests(network_class, chain_id);
CREATE INDEX IF NOT EXISTS idx_wrap_token_requests_creation_height ON wrap_token_requests(creation_momentum_height);

CREATE INDEX IF NOT EXISTS idx_unwrap_token_requests_to_address ON unwrap_token_requests(to_address);
CREATE INDEX IF NOT EXISTS idx_unwrap_token_requests_token_standard ON unwrap_token_requests(token_standard);
CREATE INDEX IF NOT EXISTS idx_unwrap_token_requests_chain ON unwrap_token_requests(network_class, chain_id);
CREATE INDEX IF NOT EXISTS idx_unwrap_token_requests_registration_height ON unwrap_token_requests(registration_momentum_height);
CREATE INDEX IF NOT EXISTS idx_unwrap_token_requests_status ON unwrap_token_requests(redeemed, revoked);
