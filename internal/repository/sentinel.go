package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type SentinelRepository struct {
	pool *pgxpool.Pool
}

func NewSentinelRepository(pool *pgxpool.Pool) *SentinelRepository {
	return &SentinelRepository{pool: pool}
}

// Upsert inserts or updates a sentinel
func (r *SentinelRepository) Upsert(ctx context.Context, s *models.Sentinel) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sentinels (owner, registration_timestamp, is_revocable, revoke_cooldown, active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (owner) DO UPDATE SET
			is_revocable = EXCLUDED.is_revocable,
			revoke_cooldown = EXCLUDED.revoke_cooldown`,
		s.Owner, s.RegistrationTimestamp, s.IsRevocable, s.RevokeCooldown, s.Active)
	return err
}

// SetInactive marks a sentinel as inactive
func (r *SentinelRepository) SetInactive(ctx context.Context, owner string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sentinels SET active = false WHERE owner = $1`,
		owner)
	return err
}

// SetInactiveBatch adds an inactive update to a batch
func (r *SentinelRepository) SetInactiveBatch(batch *pgx.Batch, owner string) {
	batch.Queue(`
		UPDATE sentinels SET active = false WHERE owner = $1`,
		owner)
}

// List returns sentinels ordered by registration timestamp descending
// (newest first). active sentinels only by default.
func (r *SentinelRepository) List(ctx context.Context, activeOnly bool, opts ListOpts) ([]*models.Sentinel, int64, error) {
	where := ""
	if activeOnly {
		where = "WHERE active = true"
	}
	query := `
		SELECT owner, registration_timestamp, is_revocable, revoke_cooldown, active,
			COUNT(*) OVER () AS total
		FROM sentinels ` + where + `
		ORDER BY registration_timestamp DESC
		LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Sentinel
		total int64
	)
	for rows.Next() {
		var s models.Sentinel
		if err := rows.Scan(&s.Owner, &s.RegistrationTimestamp, &s.IsRevocable, &s.RevokeCooldown, &s.Active, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM sentinels `+where)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// GetByOwner retrieves a sentinel by owner
func (r *SentinelRepository) GetByOwner(ctx context.Context, owner string) (*models.Sentinel, error) {
	var s models.Sentinel
	err := r.pool.QueryRow(ctx, `
		SELECT owner, registration_timestamp, is_revocable, revoke_cooldown, active
		FROM sentinels WHERE owner = $1`, owner).Scan(
		&s.Owner, &s.RegistrationTimestamp, &s.IsRevocable, &s.RevokeCooldown, &s.Active)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
