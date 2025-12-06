-- Drop all indexes
DROP INDEX IF EXISTS idx_account_blocks_address;
DROP INDEX IF EXISTS idx_account_blocks_to_address;
DROP INDEX IF EXISTS idx_account_blocks_momentum_height;
DROP INDEX IF EXISTS idx_account_blocks_token_standard;
DROP INDEX IF EXISTS idx_account_blocks_method;

DROP INDEX IF EXISTS idx_balances_token_standard;
DROP INDEX IF EXISTS idx_balances_balance;

DROP INDEX IF EXISTS idx_pillars_name;
DROP INDEX IF EXISTS idx_pillars_producer_address;
DROP INDEX IF EXISTS idx_pillars_withdraw_address;

DROP INDEX IF EXISTS idx_pillar_updates_owner_address;
DROP INDEX IF EXISTS idx_pillar_updates_producer_address;
DROP INDEX IF EXISTS idx_pillar_updates_withdraw_address;
DROP INDEX IF EXISTS idx_pillar_updates_momentum_height;

DROP INDEX IF EXISTS idx_votes_voter_address;
DROP INDEX IF EXISTS idx_votes_project_id;
DROP INDEX IF EXISTS idx_votes_phase_id;
DROP INDEX IF EXISTS idx_votes_voting_id;

DROP INDEX IF EXISTS idx_reward_transactions_address;
DROP INDEX IF EXISTS idx_reward_transactions_reward_type;
DROP INDEX IF EXISTS idx_reward_transactions_momentum_height;

DROP INDEX IF EXISTS idx_fusions_address;
DROP INDEX IF EXISTS idx_fusions_beneficiary;
DROP INDEX IF EXISTS idx_fusions_is_active;

DROP INDEX IF EXISTS idx_stakes_address;
DROP INDEX IF EXISTS idx_stakes_is_active;

DROP INDEX IF EXISTS idx_accounts_delegate;

DROP INDEX IF EXISTS idx_projects_owner;
DROP INDEX IF EXISTS idx_projects_voting_id;

DROP INDEX IF EXISTS idx_project_phases_project_id;
DROP INDEX IF EXISTS idx_project_phases_voting_id;

DROP INDEX IF EXISTS idx_momentums_timestamp;
DROP INDEX IF EXISTS idx_momentums_producer;
