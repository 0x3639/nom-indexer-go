-- Remove finality tracking fields from bridge tables
ALTER TABLE wrap_token_requests DROP COLUMN confirmations_to_finality;
ALTER TABLE unwrap_token_requests DROP COLUMN redeemable_in;
