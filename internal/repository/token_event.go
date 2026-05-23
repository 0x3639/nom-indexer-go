package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// TokenEventRepository tracks individual mint and burn events per token.
type TokenEventRepository struct {
	pool *pgxpool.Pool
}

func NewTokenEventRepository(pool *pgxpool.Pool) *TokenEventRepository {
	return &TokenEventRepository{pool: pool}
}

func (r *TokenEventRepository) InsertMint(ctx context.Context, m *models.TokenMint) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO token_mints (account_block_hash, momentum_height, momentum_timestamp,
			token_standard, issuer, receiver, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (account_block_hash) DO NOTHING`,
		m.AccountBlockHash, m.MomentumHeight, m.MomentumTimestamp,
		m.TokenStandard, m.Issuer, m.Receiver, m.Amount)
	return err
}

func (r *TokenEventRepository) InsertMintBatch(batch *pgx.Batch, m *models.TokenMint) {
	batch.Queue(`
		INSERT INTO token_mints (account_block_hash, momentum_height, momentum_timestamp,
			token_standard, issuer, receiver, amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (account_block_hash) DO NOTHING`,
		m.AccountBlockHash, m.MomentumHeight, m.MomentumTimestamp,
		m.TokenStandard, m.Issuer, m.Receiver, m.Amount)
}

func (r *TokenEventRepository) InsertBurn(ctx context.Context, b *models.TokenBurn) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO token_burns (account_block_hash, momentum_height, momentum_timestamp,
			token_standard, burner, amount)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_block_hash) DO NOTHING`,
		b.AccountBlockHash, b.MomentumHeight, b.MomentumTimestamp,
		b.TokenStandard, b.Burner, b.Amount)
	return err
}

func (r *TokenEventRepository) InsertBurnBatch(batch *pgx.Batch, b *models.TokenBurn) {
	batch.Queue(`
		INSERT INTO token_burns (account_block_hash, momentum_height, momentum_timestamp,
			token_standard, burner, amount)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_block_hash) DO NOTHING`,
		b.AccountBlockHash, b.MomentumHeight, b.MomentumTimestamp,
		b.TokenStandard, b.Burner, b.Amount)
}

// SumDailyMintsBurns returns total mints+burns for one token across a UTC
// date. The date is treated as midnight-to-midnight in UTC regardless of the
// Postgres session timezone.
func (r *TokenEventRepository) SumDailyMintsBurns(ctx context.Context, tokenStandard, date string) (mints, burns int64, err error) {
	err = r.pool.QueryRow(ctx, `
		WITH bounds AS (
			SELECT EXTRACT(EPOCH FROM ($1::date AT TIME ZONE 'UTC'))::bigint AS lo,
			       EXTRACT(EPOCH FROM (($1::date + INTERVAL '1 day') AT TIME ZONE 'UTC'))::bigint AS hi
		)
		SELECT
			COALESCE((SELECT SUM(amount) FROM token_mints, bounds
				WHERE token_standard = $2
				AND momentum_timestamp >= bounds.lo
				AND momentum_timestamp < bounds.hi), 0),
			COALESCE((SELECT SUM(amount) FROM token_burns, bounds
				WHERE token_standard = $2
				AND momentum_timestamp >= bounds.lo
				AND momentum_timestamp < bounds.hi), 0)`,
		date, tokenStandard).Scan(&mints, &burns)
	return
}
