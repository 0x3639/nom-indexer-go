package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// Upsert inserts or updates an account
func (r *AccountRepository) Upsert(ctx context.Context, a *models.Account) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO accounts (address, block_count, public_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (address) DO UPDATE SET
			block_count = $2,
			public_key = COALESCE(NULLIF($3, ''), accounts.public_key)`,
		a.Address, a.BlockCount, a.PublicKey)
	return err
}

// UpsertBatch adds an account upsert to a batch
func (r *AccountRepository) UpsertBatch(batch *pgx.Batch, a *models.Account) {
	batch.Queue(`
		INSERT INTO accounts (address, block_count, public_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (address) DO UPDATE SET
			block_count = $2,
			public_key = COALESCE(NULLIF($3, ''), accounts.public_key)`,
		a.Address, a.BlockCount, a.PublicKey)
}

// UpdateDelegate updates the delegate for an account
func (r *AccountRepository) UpdateDelegate(ctx context.Context, address, delegate string, timestamp int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE accounts
		SET delegate = $2, delegation_start_timestamp = $3
		WHERE address = $1`,
		address, delegate, timestamp)
	return err
}

// UpdateDelegateBatch adds a delegate update to a batch
func (r *AccountRepository) UpdateDelegateBatch(batch *pgx.Batch, address, delegate string, timestamp int64) {
	batch.Queue(`
		UPDATE accounts
		SET delegate = $2, delegation_start_timestamp = $3
		WHERE address = $1`,
		address, delegate, timestamp)
}

// GetByAddress retrieves an account by address
func (r *AccountRepository) GetByAddress(ctx context.Context, address string) (*models.Account, error) {
	var a models.Account
	err := r.pool.QueryRow(ctx, `
		SELECT address, block_count, public_key, delegate, delegation_start_timestamp
		FROM accounts WHERE address = $1`, address).Scan(
		&a.Address, &a.BlockCount, &a.PublicKey, &a.Delegate, &a.DelegationStartTimestamp)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
