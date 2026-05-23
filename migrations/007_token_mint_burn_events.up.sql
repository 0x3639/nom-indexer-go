-- Track every token mint and burn as a separate event row.
-- The existing tokens.total_burned counter remains but is now derived data;
-- token_burns is the source of truth for individual burn events.

CREATE TABLE IF NOT EXISTS token_mints (
    id BIGSERIAL PRIMARY KEY,
    account_block_hash TEXT NOT NULL,
    momentum_height BIGINT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    token_standard TEXT NOT NULL,
    -- The embedded contract that minted (pillar/sentinel/stake/liquidity/token
    -- for user-owned tokens). For network-owned ZNN/QSR rewards this is the
    -- reward source contract.
    issuer TEXT NOT NULL,
    receiver TEXT NOT NULL,
    amount BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_token_mints_token_standard ON token_mints(token_standard);
CREATE INDEX IF NOT EXISTS idx_token_mints_issuer ON token_mints(issuer);
CREATE INDEX IF NOT EXISTS idx_token_mints_receiver ON token_mints(receiver);
CREATE INDEX IF NOT EXISTS idx_token_mints_momentum_height ON token_mints(momentum_height);
CREATE UNIQUE INDEX IF NOT EXISTS uq_token_mints_block_hash ON token_mints(account_block_hash);

CREATE TABLE IF NOT EXISTS token_burns (
    id BIGSERIAL PRIMARY KEY,
    account_block_hash TEXT NOT NULL,
    momentum_height BIGINT NOT NULL,
    momentum_timestamp BIGINT NOT NULL,
    token_standard TEXT NOT NULL,
    burner TEXT NOT NULL,
    amount BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_token_burns_token_standard ON token_burns(token_standard);
CREATE INDEX IF NOT EXISTS idx_token_burns_burner ON token_burns(burner);
CREATE INDEX IF NOT EXISTS idx_token_burns_momentum_height ON token_burns(momentum_height);
CREATE UNIQUE INDEX IF NOT EXISTS uq_token_burns_block_hash ON token_burns(account_block_hash);
