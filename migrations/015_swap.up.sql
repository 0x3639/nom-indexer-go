-- migrations/015_swap.up.sql
-- Legacy genesis-swap retrieval events (RetrieveAssets), one row per claim.
CREATE TABLE IF NOT EXISTS swap_retrievals (
    id                  TEXT PRIMARY KEY,           -- claiming account-block hash
    address             TEXT   NOT NULL DEFAULT '', -- recipient (claimant)
    public_key          TEXT   NOT NULL DEFAULT '',
    znn_amount          BIGINT NOT NULL DEFAULT 0,
    qsr_amount          BIGINT NOT NULL DEFAULT 0,
    momentum_height     BIGINT NOT NULL DEFAULT 0,
    momentum_timestamp  BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_swap_retrievals_address ON swap_retrievals (address);

-- Remaining (unswapped) genesis balances, snapshotted from swap.getAssets.
CREATE TABLE IF NOT EXISTS swap_assets (
    key_id_hash             TEXT PRIMARY KEY,        -- 64-hex storage key
    znn                     BIGINT NOT NULL DEFAULT 0,
    qsr                     BIGINT NOT NULL DEFAULT 0,
    last_updated_timestamp  BIGINT NOT NULL DEFAULT 0
);
