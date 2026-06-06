-- migrations/014_htlcs.up.sql
-- HTLC (hash-time-locked contract) entries, indexed from account blocks.
-- An HTLC is Created by the time-locked party, Unlocked by the hash-locked
-- party with a preimage, or Reclaimed by the time-locked party after expiry.
CREATE TABLE IF NOT EXISTS htlcs (
    id                    TEXT PRIMARY KEY,           -- creating account-block hash (64-hex)
    time_locked_address   TEXT   NOT NULL DEFAULT '', -- can Reclaim after expiry (sender)
    hash_locked_address   TEXT   NOT NULL DEFAULT '', -- can Unlock with preimage
    token_standard        TEXT   NOT NULL DEFAULT '',
    amount                BIGINT NOT NULL DEFAULT 0,
    expiration_timestamp  BIGINT NOT NULL DEFAULT 0,  -- Unix seconds
    hash_type             SMALLINT NOT NULL DEFAULT 0,-- 0=SHA3, 1=SHA256
    key_max_size          SMALLINT NOT NULL DEFAULT 0,
    hash_lock             TEXT   NOT NULL DEFAULT '', -- hex-encoded
    status                SMALLINT NOT NULL DEFAULT 0,-- 0=active,1=unlocked,2=reclaimed
    preimage              TEXT   NOT NULL DEFAULT '', -- hex-encoded, set on Unlock
    creation_momentum_height    BIGINT NOT NULL DEFAULT 0,
    creation_momentum_timestamp BIGINT NOT NULL DEFAULT 0,
    settle_momentum_height      BIGINT NOT NULL DEFAULT 0, -- height of Unlock/Reclaim
    settle_momentum_timestamp   BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_htlcs_time_locked ON htlcs (time_locked_address);
CREATE INDEX IF NOT EXISTS idx_htlcs_hash_locked ON htlcs (hash_locked_address);
CREATE INDEX IF NOT EXISTS idx_htlcs_status ON htlcs (status);
