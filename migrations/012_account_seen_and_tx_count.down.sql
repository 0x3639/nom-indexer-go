ALTER TABLE accounts
    DROP COLUMN IF EXISTS tx_count,
    DROP COLUMN IF EXISTS last_seen,
    DROP COLUMN IF EXISTS first_seen;
