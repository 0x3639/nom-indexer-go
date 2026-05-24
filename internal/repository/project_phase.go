package repository

import (
	"context"
	"fmt"

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

// GetIDsCreatedAtOrAfter returns phase IDs whose creation_timestamp is >= the
// given timestamp. Used for pillar voting-activity calculation.
func (r *ProjectPhaseRepository) GetIDsCreatedAtOrAfter(ctx context.Context, timestamp int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM project_phases WHERE creation_timestamp >= $1`, timestamp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListByProject returns every phase of a project, ordered by creation
// timestamp ascending (phase 1 first).
func (r *ProjectPhaseRepository) ListByProject(ctx context.Context, projectID string) ([]*models.ProjectPhase, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, voting_id, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, accepted_timestamp,
			status, yes_votes, no_votes, total_votes
		FROM project_phases WHERE project_id = $1
		ORDER BY creation_timestamp ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.ProjectPhase
	for rows.Next() {
		var p models.ProjectPhase
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.VotingID, &p.Name, &p.Description, &p.URL,
			&p.ZnnFundsNeeded, &p.QsrFundsNeeded, &p.CreationTimestamp, &p.AcceptedTimestamp,
			&p.Status, &p.YesVotes, &p.NoVotes, &p.TotalVotes); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
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
