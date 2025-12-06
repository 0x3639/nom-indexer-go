package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type ProjectPhaseRepository struct {
	pool *pgxpool.Pool
}

func NewProjectPhaseRepository(pool *pgxpool.Pool) *ProjectPhaseRepository {
	return &ProjectPhaseRepository{pool: pool}
}

// Upsert inserts or updates a project phase
func (r *ProjectPhaseRepository) Upsert(ctx context.Context, p *models.ProjectPhase) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO project_phases (id, project_id, voting_id, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, accepted_timestamp,
			status, yes_votes, no_votes, total_votes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
			accepted_timestamp = EXCLUDED.accepted_timestamp,
			status = EXCLUDED.status,
			yes_votes = EXCLUDED.yes_votes,
			no_votes = EXCLUDED.no_votes,
			total_votes = EXCLUDED.total_votes`,
		p.ID, p.ProjectID, p.VotingID, p.Name, p.Description, p.URL,
		p.ZnnFundsNeeded, p.QsrFundsNeeded, p.CreationTimestamp, p.AcceptedTimestamp,
		p.Status, p.YesVotes, p.NoVotes, p.TotalVotes)
	return err
}

// GetProjectAndPhaseIDFromVotingID retrieves project and phase IDs from voting ID
func (r *ProjectPhaseRepository) GetProjectAndPhaseIDFromVotingID(ctx context.Context, votingID string) (string, string, error) {
	var projectID, phaseID string
	err := r.pool.QueryRow(ctx, `
		SELECT project_id, id FROM project_phases WHERE voting_id = $1`, votingID).Scan(&projectID, &phaseID)
	if err != nil {
		return "", "", err
	}
	return projectID, phaseID, nil
}
