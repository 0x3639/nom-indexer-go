package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
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

// GetIDsCreatedAtOrAfter returns project IDs whose creation_timestamp is >=
// the given timestamp. Used to compute the set of proposals a pillar was
// eligible to vote on after it spawned.
func (r *ProjectRepository) GetIDsCreatedAtOrAfter(ctx context.Context, timestamp int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM projects WHERE creation_timestamp >= $1`, timestamp)
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

// GetByID returns a single project.
func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	var p models.Project
	err := r.pool.QueryRow(ctx, `
		SELECT id, voting_id, owner, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, last_update_timestamp,
			status, yes_votes, no_votes, total_votes
		FROM projects WHERE id = $1`, id).Scan(
		&p.ID, &p.VotingID, &p.Owner, &p.Name, &p.Description, &p.URL,
		&p.ZnnFundsNeeded, &p.QsrFundsNeeded, &p.CreationTimestamp, &p.LastUpdateTimestamp,
		&p.Status, &p.YesVotes, &p.NoVotes, &p.TotalVotes)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns projects ordered by creation_timestamp descending.
func (r *ProjectRepository) List(ctx context.Context, opts ListOpts) ([]*models.Project, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, voting_id, owner, name, description, url,
			znn_funds_needed, qsr_funds_needed, creation_timestamp, last_update_timestamp,
			status, yes_votes, no_votes, total_votes,
			COUNT(*) OVER () AS total
		FROM projects
		ORDER BY creation_timestamp DESC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Project
		total int64
	)
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.VotingID, &p.Owner, &p.Name, &p.Description, &p.URL,
			&p.ZnnFundsNeeded, &p.QsrFundsNeeded, &p.CreationTimestamp, &p.LastUpdateTimestamp,
			&p.Status, &p.YesVotes, &p.NoVotes, &p.TotalVotes, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM projects`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
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
