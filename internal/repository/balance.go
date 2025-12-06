package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type BalanceRepository struct {
	pool *pgxpool.Pool
}

func NewBalanceRepository(pool *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{pool: pool}
}

// Upsert inserts or updates a balance
func (r *BalanceRepository) Upsert(ctx context.Context, b *models.Balance) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO balances (address, token_standard, balance)
		VALUES ($1, $2, $3)
		ON CONFLICT (address, token_standard) DO UPDATE SET balance = $3`,
		b.Address, b.TokenStandard, b.Balance)
	return err
}

// UpsertBatch adds a balance upsert to a batch
func (r *BalanceRepository) UpsertBatch(batch *pgx.Batch, b *models.Balance) {
	batch.Queue(`
		INSERT INTO balances (address, token_standard, balance)
		VALUES ($1, $2, $3)
		ON CONFLICT (address, token_standard) DO UPDATE SET balance = $3`,
		b.Address, b.TokenStandard, b.Balance)
}

// GetByAddressAndToken retrieves a balance by address and token standard
func (r *BalanceRepository) GetByAddressAndToken(ctx context.Context, address, tokenStandard string) (*models.Balance, error) {
	var b models.Balance
	err := r.pool.QueryRow(ctx, `
		SELECT address, token_standard, balance
		FROM balances WHERE address = $1 AND token_standard = $2`,
		address, tokenStandard).Scan(&b.Address, &b.TokenStandard, &b.Balance)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// GetHolderCount returns the count of holders with balance > 0 for a token
func (r *BalanceRepository) GetHolderCount(ctx context.Context, tokenStandard string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM balances
		WHERE token_standard = $1 AND balance > 0`,
		tokenStandard).Scan(&count)
	return count, err
}
