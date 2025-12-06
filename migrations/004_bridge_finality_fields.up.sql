-- Add finality tracking fields to bridge tables
ALTER TABLE wrap_token_requests ADD COLUMN confirmations_to_finality INT NOT NULL DEFAULT 0;
ALTER TABLE unwrap_token_requests ADD COLUMN redeemable_in BIGINT NOT NULL DEFAULT 0;
