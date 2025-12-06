package repository

import (
	"context"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VoteRepository struct {
	pool *pgxpool.Pool
}

func NewVoteRepository(pool *pgxpool.Pool) *VoteRepository {
	return &VoteRepository{pool: pool}
}

// Insert inserts a vote
func (r *VoteRepository) Insert(ctx context.Context, v *models.Vote) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO votes (momentum_hash, momentum_timestamp, momentum_height,
			voter_address, project_id, phase_id, voting_id, vote)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		v.MomentumHash, v.MomentumTimestamp, v.MomentumHeight,
		v.VoterAddress, v.ProjectID, v.PhaseID, v.VotingID, v.Vote)
	return err
}

// InsertBatch adds a vote insert to a batch
func (r *VoteRepository) InsertBatch(batch *pgx.Batch, v *models.Vote) {
	batch.Queue(`
		INSERT INTO votes (momentum_hash, momentum_timestamp, momentum_height,
			voter_address, project_id, phase_id, voting_id, vote)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		v.MomentumHash, v.MomentumTimestamp, v.MomentumHeight,
		v.VoterAddress, v.ProjectID, v.PhaseID, v.VotingID, v.Vote)
}

// GetVoteCountForProjects counts distinct projects voted on by a voter
func (r *VoteRepository) GetVoteCountForProjects(ctx context.Context, voterAddress string, projectIDs []string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT project_id) FROM votes
		WHERE voter_address = $1 AND project_id = ANY($2) AND phase_id = ''`,
		voterAddress, projectIDs).Scan(&count)
	return count, err
}

// GetVoteCountForPhases counts distinct phases voted on by a voter
func (r *VoteRepository) GetVoteCountForPhases(ctx context.Context, voterAddress string, phaseIDs []string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT phase_id) FROM votes
		WHERE voter_address = $1 AND phase_id = ANY($2)`,
		voterAddress, phaseIDs).Scan(&count)
	return count, err
}
