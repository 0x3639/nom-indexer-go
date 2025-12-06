-- Revert log_index back to INT (may fail if values exceed INT range)
ALTER TABLE unwrap_token_requests ALTER COLUMN log_index TYPE INT;
