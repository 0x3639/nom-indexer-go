-- Delegation history table: each row is one delegation interval.
-- A delegator switching pillars closes the prior row (ended_at) and opens a
-- new one. Mirrors zenonhub's nom_delegations table.
CREATE TABLE IF NOT EXISTS delegations (
    id BIGSERIAL PRIMARY KEY,
    delegator_address TEXT NOT NULL,
    pillar_owner_address TEXT NOT NULL,
    started_at BIGINT NOT NULL,
    ended_at BIGINT
);

CREATE INDEX IF NOT EXISTS idx_delegations_delegator ON delegations(delegator_address);
CREATE INDEX IF NOT EXISTS idx_delegations_pillar ON delegations(pillar_owner_address);
CREATE INDEX IF NOT EXISTS idx_delegations_open ON delegations(delegator_address) WHERE ended_at IS NULL;
