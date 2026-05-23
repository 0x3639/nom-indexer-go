ALTER TABLE balances DROP COLUMN IF EXISTS last_updated_timestamp;

DROP INDEX IF EXISTS idx_accounts_last_active_at;
DROP INDEX IF EXISTS idx_accounts_first_active_at;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS last_active_at,
    DROP COLUMN IF EXISTS first_active_at,
    DROP COLUMN IF EXISTS qsr_received,
    DROP COLUMN IF EXISTS qsr_sent,
    DROP COLUMN IF EXISTS znn_received,
    DROP COLUMN IF EXISTS znn_sent,
    DROP COLUMN IF EXISTS genesis_qsr_balance,
    DROP COLUMN IF EXISTS genesis_znn_balance;
