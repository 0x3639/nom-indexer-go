-- Remove duplicate votes, keeping only the latest (highest momentum_height) per voter+voting_id
DELETE FROM votes
WHERE id NOT IN (
    SELECT DISTINCT ON (voter_address, voting_id)
           id
    FROM votes
    ORDER BY voter_address, voting_id, momentum_height DESC
);

-- Add unique constraint so only one vote per voter per voting_id is stored
ALTER TABLE votes ADD CONSTRAINT uq_votes_voter_voting UNIQUE (voter_address, voting_id);
