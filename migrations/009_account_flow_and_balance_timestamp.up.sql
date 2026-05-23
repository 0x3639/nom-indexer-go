-- Account-level flow metrics mirrored from zenonhub: genesis balances,
-- directional ZNN/QSR send/receive totals, and first/last activity.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS genesis_znn_balance BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS genesis_qsr_balance BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS znn_sent BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS znn_received BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS qsr_sent BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS qsr_received BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS first_active_at BIGINT,
    ADD COLUMN IF NOT EXISTS last_active_at BIGINT;

CREATE INDEX IF NOT EXISTS idx_accounts_first_active_at ON accounts(first_active_at);
CREATE INDEX IF NOT EXISTS idx_accounts_last_active_at ON accounts(last_active_at);

-- Track when a balance was last updated so "stale" balances are visible.
ALTER TABLE balances
    ADD COLUMN IF NOT EXISTS last_updated_timestamp BIGINT NOT NULL DEFAULT 0;
