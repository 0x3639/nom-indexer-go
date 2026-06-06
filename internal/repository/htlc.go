package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type HtlcRepository struct {
	pool *pgxpool.Pool
}

func NewHtlcRepository(pool *pgxpool.Pool) *HtlcRepository {
	return &HtlcRepository{pool: pool}
}

const htlcCols = `id, time_locked_address, hash_locked_address, token_standard, amount,
	expiration_timestamp, hash_type, key_max_size, hash_lock, status, preimage,
	creation_momentum_height, creation_momentum_timestamp,
	settle_momentum_height, settle_momentum_timestamp`

// Insert inserts an HTLC entry. Idempotent via ON CONFLICT (id) DO NOTHING.
func (r *HtlcRepository) Insert(ctx context.Context, h *models.Htlc) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO htlcs (`+htlcCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO NOTHING`,
		h.ID, h.TimeLockedAddress, h.HashLockedAddress, h.TokenStandard, h.Amount,
		h.ExpirationTimestamp, h.HashType, h.KeyMaxSize, h.HashLock, h.Status, h.Preimage,
		h.CreationMomentumHeight, h.CreationMomentumTimestamp,
		h.SettleMomentumHeight, h.SettleMomentumTimestamp)
	return err
}

// InsertBatch enqueues an HTLC Create on the per-momentum batch.
func (r *HtlcRepository) InsertBatch(batch *pgx.Batch, h *models.Htlc) {
	batch.Queue(`
		INSERT INTO htlcs (`+htlcCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO NOTHING`,
		h.ID, h.TimeLockedAddress, h.HashLockedAddress, h.TokenStandard, h.Amount,
		h.ExpirationTimestamp, h.HashType, h.KeyMaxSize, h.HashLock, h.Status, h.Preimage,
		h.CreationMomentumHeight, h.CreationMomentumTimestamp,
		h.SettleMomentumHeight, h.SettleMomentumTimestamp)
}

// Settle marks an HTLC unlocked (with preimage) or reclaimed.
func (r *HtlcRepository) Settle(ctx context.Context, id string, status int16, preimage string, height, ts int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE htlcs SET status = $2, preimage = $3,
			settle_momentum_height = $4, settle_momentum_timestamp = $5
		WHERE id = $1`,
		id, status, preimage, height, ts)
	return err
}

// SettleBatch enqueues an Unlock/Reclaim settle on the per-momentum batch.
func (r *HtlcRepository) SettleBatch(batch *pgx.Batch, id string, status int16, preimage string, height, ts int64) {
	batch.Queue(`
		UPDATE htlcs SET status = $2, preimage = $3,
			settle_momentum_height = $4, settle_momentum_timestamp = $5
		WHERE id = $1`,
		id, status, preimage, height, ts)
}

// GetByID retrieves an HTLC by ID.
func (r *HtlcRepository) GetByID(ctx context.Context, id string) (*models.Htlc, error) {
	var h models.Htlc
	err := r.pool.QueryRow(ctx, `SELECT `+htlcCols+` FROM htlcs WHERE id = $1`, id).Scan(
		&h.ID, &h.TimeLockedAddress, &h.HashLockedAddress, &h.TokenStandard, &h.Amount,
		&h.ExpirationTimestamp, &h.HashType, &h.KeyMaxSize, &h.HashLock, &h.Status, &h.Preimage,
		&h.CreationMomentumHeight, &h.CreationMomentumTimestamp,
		&h.SettleMomentumHeight, &h.SettleMomentumTimestamp)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// List returns HTLCs ordered by creation_momentum_height descending.
func (r *HtlcRepository) List(ctx context.Context, opts ListOpts) ([]*models.Htlc, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+htlcCols+`, COUNT(*) OVER () AS total
		FROM htlcs ORDER BY creation_momentum_height DESC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Htlc
		total int64
	)
	for rows.Next() {
		var h models.Htlc
		if err := rows.Scan(&h.ID, &h.TimeLockedAddress, &h.HashLockedAddress, &h.TokenStandard, &h.Amount,
			&h.ExpirationTimestamp, &h.HashType, &h.KeyMaxSize, &h.HashLock, &h.Status, &h.Preimage,
			&h.CreationMomentumHeight, &h.CreationMomentumTimestamp,
			&h.SettleMomentumHeight, &h.SettleMomentumTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM htlcs`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
