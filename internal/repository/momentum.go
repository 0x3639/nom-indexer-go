package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type MomentumRepository struct {
	pool *pgxpool.Pool
}

func NewMomentumRepository(pool *pgxpool.Pool) *MomentumRepository {
	return &MomentumRepository{pool: pool}
}

// GetLatestHeight returns the latest indexed momentum height
func (r *MomentumRepository) GetLatestHeight(ctx context.Context) (uint64, error) {
	var height *int64
	err := r.pool.QueryRow(ctx, "SELECT MAX(height) FROM momentums").Scan(&height)
	if err != nil {
		return 0, err
	}
	if height == nil {
		return 0, nil
	}
	return uint64(*height), nil
}

// Insert inserts a momentum, ignoring conflicts
func (r *MomentumRepository) Insert(ctx context.Context, m *models.Momentum) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (height) DO NOTHING`,
		m.Height, m.Hash, m.Timestamp, m.TxCount, m.Producer, m.ProducerOwner, m.ProducerName)
	return err
}

// InsertBatch inserts multiple momentums using a batch
func (r *MomentumRepository) InsertBatch(ctx context.Context, batch *pgx.Batch, m *models.Momentum) {
	batch.Queue(`
		INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (height) DO NOTHING`,
		m.Height, m.Hash, m.Timestamp, m.TxCount, m.Producer, m.ProducerOwner, m.ProducerName)
}

// GetByHeight retrieves a momentum by height
func (r *MomentumRepository) GetByHeight(ctx context.Context, height uint64) (*models.Momentum, error) {
	var m models.Momentum
	err := r.pool.QueryRow(ctx, `
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums WHERE height = $1`, height).Scan(
		&m.Height, &m.Hash, &m.Timestamp, &m.TxCount, &m.Producer, &m.ProducerOwner, &m.ProducerName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetLatest returns the most recent momentum (highest height). Returns
// pgx.ErrNoRows on an empty table.
func (r *MomentumRepository) GetLatest(ctx context.Context) (*models.Momentum, error) {
	var m models.Momentum
	err := r.pool.QueryRow(ctx, `
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums ORDER BY height DESC LIMIT 1`).Scan(
		&m.Height, &m.Hash, &m.Timestamp, &m.TxCount, &m.Producer, &m.ProducerOwner, &m.ProducerName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List returns momentums ordered by height (sort = "asc" or default desc),
// along with the total count for pagination metadata.
func (r *MomentumRepository) List(ctx context.Context, opts ListOpts) ([]*models.Momentum, int64, error) {
	query := fmt.Sprintf(`
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name,
			COUNT(*) OVER () AS total
		FROM momentums
		ORDER BY height %s
		LIMIT $1 OFFSET $2`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Momentum
		total int64
	)
	for rows.Next() {
		var m models.Momentum
		if err := rows.Scan(&m.Height, &m.Hash, &m.Timestamp, &m.TxCount,
			&m.Producer, &m.ProducerOwner, &m.ProducerName, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM momentums`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// ListByProducer returns momentums whose producer matches the address,
// newest-first. Used to power "blocks produced by pillar" queries.
func (r *MomentumRepository) ListByProducer(ctx context.Context, producer string, opts ListOpts) ([]*models.Momentum, int64, error) {
	if producer == "" {
		return nil, 0, errors.New("producer is required")
	}
	query := fmt.Sprintf(`
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name,
			COUNT(*) OVER () AS total
		FROM momentums
		WHERE producer = $1
		ORDER BY height %s
		LIMIT $2 OFFSET $3`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, producer, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Momentum
		total int64
	)
	for rows.Next() {
		var m models.Momentum
		if err := rows.Scan(&m.Height, &m.Hash, &m.Timestamp, &m.TxCount,
			&m.Producer, &m.ProducerOwner, &m.ProducerName, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM momentums WHERE producer = $1`, producer)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
