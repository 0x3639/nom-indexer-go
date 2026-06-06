package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type SwapRepository struct {
	pool *pgxpool.Pool
}

func NewSwapRepository(pool *pgxpool.Pool) *SwapRepository {
	return &SwapRepository{pool: pool}
}

const swapRetrievalCols = `id, address, public_key, znn_amount, qsr_amount,
	momentum_height, momentum_timestamp`

const swapAssetCols = `key_id_hash, znn, qsr, last_updated_timestamp`

// InsertRetrieval inserts a RetrieveAssets claim. Idempotent via
// ON CONFLICT (id) DO NOTHING.
func (r *SwapRepository) InsertRetrieval(ctx context.Context, s *models.SwapRetrieval) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO swap_retrievals (`+swapRetrievalCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.PublicKey, s.ZnnAmount, s.QsrAmount,
		s.MomentumHeight, s.MomentumTimestamp)
	return err
}

// InsertRetrievalBatch enqueues a RetrieveAssets claim on the per-momentum batch.
func (r *SwapRepository) InsertRetrievalBatch(batch *pgx.Batch, s *models.SwapRetrieval) {
	batch.Queue(`
		INSERT INTO swap_retrievals (`+swapRetrievalCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.PublicKey, s.ZnnAmount, s.QsrAmount,
		s.MomentumHeight, s.MomentumTimestamp)
}

// UpsertAsset writes the remaining unswapped balance for a keyIdHash,
// overwriting any prior snapshot.
func (r *SwapRepository) UpsertAsset(ctx context.Context, a *models.SwapAsset) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO swap_assets (`+swapAssetCols+`)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (key_id_hash) DO UPDATE SET
			znn = EXCLUDED.znn,
			qsr = EXCLUDED.qsr,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		a.KeyIDHash, a.Znn, a.Qsr, a.LastUpdatedTimestamp)
	return err
}

// GetAsset retrieves a remaining-balance snapshot by keyIdHash.
func (r *SwapRepository) GetAsset(ctx context.Context, keyIDHash string) (*models.SwapAsset, error) {
	var a models.SwapAsset
	err := r.pool.QueryRow(ctx, `SELECT `+swapAssetCols+` FROM swap_assets WHERE key_id_hash = $1`, keyIDHash).Scan(
		&a.KeyIDHash, &a.Znn, &a.Qsr, &a.LastUpdatedTimestamp)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListRetrievals returns swap retrievals ordered by momentum_height descending.
func (r *SwapRepository) ListRetrievals(ctx context.Context, opts ListOpts) ([]*models.SwapRetrieval, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+swapRetrievalCols+`, COUNT(*) OVER () AS total
		FROM swap_retrievals ORDER BY momentum_height DESC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.SwapRetrieval
		total int64
	)
	for rows.Next() {
		var s models.SwapRetrieval
		if err := rows.Scan(&s.ID, &s.Address, &s.PublicKey, &s.ZnnAmount, &s.QsrAmount,
			&s.MomentumHeight, &s.MomentumTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM swap_retrievals`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
