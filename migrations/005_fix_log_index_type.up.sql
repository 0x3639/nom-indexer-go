-- Change log_index from INT to BIGINT to handle large values
ALTER TABLE unwrap_token_requests ALTER COLUMN log_index TYPE BIGINT;
