package repository

import (
	"context"

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
