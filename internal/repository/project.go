package repository

import (
	"context"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

// Upsert inserts or updates a project
func (r *ProjectRepository) Upsert(ctx context.Context, p *models.Project) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO projects (id, voting_id, owner, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, last_update_timestamp,
			status, yes_votes, no_votes, total_votes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
			last_update_timestamp = EXCLUDED.last_update_timestamp,
			status = EXCLUDED.status,
			yes_votes = EXCLUDED.yes_votes,
			no_votes = EXCLUDED.no_votes,
			total_votes = EXCLUDED.total_votes`,
		p.ID, p.VotingID, p.Owner, p.Name, p.Description, p.URL,
		p.ZnnFundsNeeded, p.QsrFundsNeeded, p.CreationTimestamp, p.LastUpdateTimestamp,
		p.Status, p.YesVotes, p.NoVotes, p.TotalVotes)
	return err
}

// GetIDFromVotingID retrieves a project ID from its voting ID
func (r *ProjectRepository) GetIDFromVotingID(ctx context.Context, votingID string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM projects WHERE voting_id = $1`, votingID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetAll retrieves all projects
func (r *ProjectRepository) GetAll(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, voting_id, owner, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, last_update_timestamp,
			status, yes_votes, no_votes, total_votes
		FROM projects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		var p models.Project
		err := rows.Scan(&p.ID, &p.VotingID, &p.Owner, &p.Name, &p.Description, &p.URL,
			&p.ZnnFundsNeeded, &p.QsrFundsNeeded, &p.CreationTimestamp, &p.LastUpdateTimestamp,
			&p.Status, &p.YesVotes, &p.NoVotes, &p.TotalVotes)
		if err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, nil
}
