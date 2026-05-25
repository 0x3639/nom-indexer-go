-- Per-account counters maintained incrementally as account_blocks are indexed.
--
-- first_seen / last_seen: unix seconds of the earliest / most recent block
-- where the address appears in EITHER role (sender == account_blocks.address
-- OR recipient == account_blocks.to_address). Distinct from the
-- existing first_active_at/last_active_at columns, which only track blocks
-- where the address is the chain owner (sender on sends, recipient on
-- receives) and thus miss sends that have not yet been claimed.
--
-- tx_count: count of those same blocks. Matches the pagination.total
-- returned by GET /api/v1/accounts/{address}/transactions (which filters
-- WHERE address = $1 OR to_address = $1). Distinct from block_count, which
-- is the per-account chain height (sender-only).
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS first_seen BIGINT,
    ADD COLUMN IF NOT EXISTS last_seen  BIGINT,
    ADD COLUMN IF NOT EXISTS tx_count   BIGINT NOT NULL DEFAULT 0;

-- Backfill: every existing account_block contributes once per distinct
-- address it touches. UNION (not UNION ALL) collapses the rare self-send
-- case where address == to_address into a single appearance per block,
-- matching the /transactions row count.
--
-- Insert stubs for addresses that only appear as to_address (recipients
-- that have not submitted a receive block yet) so the counters survive
-- a future query for that address.
INSERT INTO accounts (address, block_count, public_key, first_seen, last_seen, tx_count)
SELECT addr,
       0 AS block_count,
       '' AS public_key,
       MIN(momentum_timestamp) AS first_seen,
       MAX(momentum_timestamp) AS last_seen,
       COUNT(*) AS tx_count
FROM (
    SELECT hash, address AS addr, momentum_timestamp
    FROM account_blocks
    UNION
    SELECT hash, to_address AS addr, momentum_timestamp
    FROM account_blocks
    WHERE to_address IS NOT NULL AND to_address <> ''
) appearances
GROUP BY addr
ON CONFLICT (address) DO UPDATE SET
    first_seen = EXCLUDED.first_seen,
    last_seen  = EXCLUDED.last_seen,
    tx_count   = EXCLUDED.tx_count;
