package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type FusionRepository struct {
	pool *pgxpool.Pool
}

func NewFusionRepository(pool *pgxpool.Pool) *FusionRepository {
	return &FusionRepository{pool: pool}
}

// Insert inserts a fusion entry
func (r *FusionRepository) Insert(ctx context.Context, f *models.Fusion) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO fusions (id, address, beneficiary, momentum_hash, momentum_timestamp,
			momentum_height, qsr_amount, expiration_height, is_active, cancel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING`,
		f.ID, f.Address, f.Beneficiary, f.MomentumHash, f.MomentumTimestamp,
		f.MomentumHeight, f.QsrAmount, f.ExpirationHeight, f.IsActive, f.CancelID)
	return err
}

// InsertBatch adds a fusion insert to a batch
func (r *FusionRepository) InsertBatch(batch *pgx.Batch, f *models.Fusion) {
	batch.Queue(`
		INSERT INTO fusions (id, address, beneficiary, momentum_hash, momentum_timestamp,
			momentum_height, qsr_amount, expiration_height, is_active, cancel_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING`,
		f.ID, f.Address, f.Beneficiary, f.MomentumHash, f.MomentumTimestamp,
		f.MomentumHeight, f.QsrAmount, f.ExpirationHeight, f.IsActive, f.CancelID)
}

// SetInactive marks a fusion as inactive by cancel ID and address
func (r *FusionRepository) SetInactive(ctx context.Context, cancelID, address string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE fusions SET is_active = false
		WHERE cancel_id = $1 AND address = $2`,
		cancelID, address)
	return err
}

// SetInactiveBatch adds an inactive update to a batch
func (r *FusionRepository) SetInactiveBatch(batch *pgx.Batch, cancelID, address string) {
	batch.Queue(`
		UPDATE fusions SET is_active = false
		WHERE cancel_id = $1 AND address = $2`,
		cancelID, address)
}

// GetByID retrieves a fusion by ID
func (r *FusionRepository) GetByID(ctx context.Context, id string) (*models.Fusion, error) {
	var f models.Fusion
	err := r.pool.QueryRow(ctx, `
		SELECT id, address, beneficiary, momentum_hash, momentum_timestamp,
			momentum_height, qsr_amount, expiration_height, is_active, cancel_id
		FROM fusions WHERE id = $1`, id).Scan(
		&f.ID, &f.Address, &f.Beneficiary, &f.MomentumHash, &f.MomentumTimestamp,
		&f.MomentumHeight, &f.QsrAmount, &f.ExpirationHeight, &f.IsActive, &f.CancelID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
