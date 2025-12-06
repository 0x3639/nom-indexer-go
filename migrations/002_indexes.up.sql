-- Performance indexes for common queries

-- Account blocks indexes
CREATE INDEX IF NOT EXISTS idx_account_blocks_address ON account_blocks(address);
CREATE INDEX IF NOT EXISTS idx_account_blocks_to_address ON account_blocks(to_address);
CREATE INDEX IF NOT EXISTS idx_account_blocks_momentum_height ON account_blocks(momentum_height);
CREATE INDEX IF NOT EXISTS idx_account_blocks_token_standard ON account_blocks(token_standard);
CREATE INDEX IF NOT EXISTS idx_account_blocks_method ON account_blocks(method);

-- Balances indexes
CREATE INDEX IF NOT EXISTS idx_balances_token_standard ON balances(token_standard);
CREATE INDEX IF NOT EXISTS idx_balances_balance ON balances(balance) WHERE balance > 0;

-- Pillars indexes
CREATE INDEX IF NOT EXISTS idx_pillars_name ON pillars(name);
CREATE INDEX IF NOT EXISTS idx_pillars_producer_address ON pillars(producer_address);
CREATE INDEX IF NOT EXISTS idx_pillars_withdraw_address ON pillars(withdraw_address);

-- Pillar updates indexes
CREATE INDEX IF NOT EXISTS idx_pillar_updates_owner_address ON pillar_updates(owner_address);
CREATE INDEX IF NOT EXISTS idx_pillar_updates_producer_address ON pillar_updates(producer_address);
CREATE INDEX IF NOT EXISTS idx_pillar_updates_withdraw_address ON pillar_updates(withdraw_address);
CREATE INDEX IF NOT EXISTS idx_pillar_updates_momentum_height ON pillar_updates(momentum_height);

-- Votes indexes
CREATE INDEX IF NOT EXISTS idx_votes_voter_address ON votes(voter_address);
CREATE INDEX IF NOT EXISTS idx_votes_project_id ON votes(project_id);
CREATE INDEX IF NOT EXISTS idx_votes_phase_id ON votes(phase_id);
CREATE INDEX IF NOT EXISTS idx_votes_voting_id ON votes(voting_id);

-- Reward transactions indexes
CREATE INDEX IF NOT EXISTS idx_reward_transactions_address ON reward_transactions(address);
CREATE INDEX IF NOT EXISTS idx_reward_transactions_reward_type ON reward_transactions(reward_type);
CREATE INDEX IF NOT EXISTS idx_reward_transactions_momentum_height ON reward_transactions(momentum_height);

-- Fusions indexes
CREATE INDEX IF NOT EXISTS idx_fusions_address ON fusions(address);
CREATE INDEX IF NOT EXISTS idx_fusions_beneficiary ON fusions(beneficiary);
CREATE INDEX IF NOT EXISTS idx_fusions_is_active ON fusions(is_active);

-- Stakes indexes
CREATE INDEX IF NOT EXISTS idx_stakes_address ON stakes(address);
CREATE INDEX IF NOT EXISTS idx_stakes_is_active ON stakes(is_active);

-- Accounts indexes
CREATE INDEX IF NOT EXISTS idx_accounts_delegate ON accounts(delegate);

-- Projects indexes
CREATE INDEX IF NOT EXISTS idx_projects_owner ON projects(owner);
CREATE INDEX IF NOT EXISTS idx_projects_voting_id ON projects(voting_id);

-- Project phases indexes
CREATE INDEX IF NOT EXISTS idx_project_phases_project_id ON project_phases(project_id);
CREATE INDEX IF NOT EXISTS idx_project_phases_voting_id ON project_phases(voting_id);

-- Momentums indexes
CREATE INDEX IF NOT EXISTS idx_momentums_timestamp ON momentums(timestamp);
CREATE INDEX IF NOT EXISTS idx_momentums_producer ON momentums(producer);
