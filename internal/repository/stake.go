package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type StakeRepository struct {
	pool *pgxpool.Pool
}

func NewStakeRepository(pool *pgxpool.Pool) *StakeRepository {
	return &StakeRepository{pool: pool}
}

// Insert inserts a stake entry
func (r *StakeRepository) Insert(ctx context.Context, s *models.Stake) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO stakes (id, address, start_timestamp, expiration_timestamp, znn_amount,
			duration_in_sec, is_active, cancel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.StartTimestamp, s.ExpirationTimestamp, s.ZnnAmount,
		s.DurationInSec, s.IsActive, s.CancelID)
	return err
}

// InsertBatch adds a stake insert to a batch
func (r *StakeRepository) InsertBatch(batch *pgx.Batch, s *models.Stake) {
	batch.Queue(`
		INSERT INTO stakes (id, address, start_timestamp, expiration_timestamp, znn_amount,
			duration_in_sec, is_active, cancel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.StartTimestamp, s.ExpirationTimestamp, s.ZnnAmount,
		s.DurationInSec, s.IsActive, s.CancelID)
}

// SetInactive marks a stake as inactive by cancel ID and address
func (r *StakeRepository) SetInactive(ctx context.Context, cancelID, address string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE stakes SET is_active = false
		WHERE cancel_id = $1 AND address = $2`,
		cancelID, address)
	return err
}

// SetInactiveBatch adds an inactive update to a batch
func (r *StakeRepository) SetInactiveBatch(batch *pgx.Batch, cancelID, address string) {
	batch.Queue(`
		UPDATE stakes SET is_active = false
		WHERE cancel_id = $1 AND address = $2`,
		cancelID, address)
}

// List returns stakes ordered by start_timestamp descending. activeOnly
// filters to is_active = true.
func (r *StakeRepository) List(ctx context.Context, activeOnly bool, opts ListOpts) ([]*models.Stake, int64, error) {
	where := ""
	if activeOnly {
		where = "WHERE is_active = true"
	}
	query := `
		SELECT id, address, start_timestamp, expiration_timestamp, znn_amount,
			duration_in_sec, is_active, cancel_id, COUNT(*) OVER () AS total
		FROM stakes ` + where + `
		ORDER BY start_timestamp DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Stake
		total int64
	)
	for rows.Next() {
		var s models.Stake
		if err := rows.Scan(&s.ID, &s.Address, &s.StartTimestamp, &s.ExpirationTimestamp, &s.ZnnAmount,
			&s.DurationInSec, &s.IsActive, &s.CancelID, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM stakes `+where)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// ListByAddress returns stakes owned by the given address.
func (r *StakeRepository) ListByAddress(ctx context.Context, address string, activeOnly bool, opts ListOpts) ([]*models.Stake, int64, error) {
	if address == "" {
		return nil, 0, fmt.Errorf("address is required")
	}
	where := "WHERE address = $1"
	if activeOnly {
		where += " AND is_active = true"
	}
	query := `
		SELECT id, address, start_timestamp, expiration_timestamp, znn_amount,
			duration_in_sec, is_active, cancel_id, COUNT(*) OVER () AS total
		FROM stakes ` + where + `
		ORDER BY start_timestamp DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, address, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Stake
		total int64
	)
	for rows.Next() {
		var s models.Stake
		if err := rows.Scan(&s.ID, &s.Address, &s.StartTimestamp, &s.ExpirationTimestamp, &s.ZnnAmount,
			&s.DurationInSec, &s.IsActive, &s.CancelID, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM stakes `+where, address)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// GetByID retrieves a stake by ID
func (r *StakeRepository) GetByID(ctx context.Context, id string) (*models.Stake, error) {
	var s models.Stake
	err := r.pool.QueryRow(ctx, `
		SELECT id, address, start_timestamp, expiration_timestamp, znn_amount,
			duration_in_sec, is_active, cancel_id
		FROM stakes WHERE id = $1`, id).Scan(
		&s.ID, &s.Address, &s.StartTimestamp, &s.ExpirationTimestamp, &s.ZnnAmount,
		&s.DurationInSec, &s.IsActive, &s.CancelID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
