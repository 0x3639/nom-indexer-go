package repository

import (
	"context"
	"fmt"

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
		INSERT INTO balances (address, token_standard, balance, last_updated_timestamp)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address, token_standard) DO UPDATE SET
			balance = EXCLUDED.balance,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		b.Address, b.TokenStandard, b.Balance, b.LastUpdatedTimestamp)
	return err
}

// UpsertBatch adds a balance upsert to a batch
func (r *BalanceRepository) UpsertBatch(batch *pgx.Batch, b *models.Balance) {
	batch.Queue(`
		INSERT INTO balances (address, token_standard, balance, last_updated_timestamp)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address, token_standard) DO UPDATE SET
			balance = EXCLUDED.balance,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		b.Address, b.TokenStandard, b.Balance, b.LastUpdatedTimestamp)
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

// ListByAddress returns every (token, balance) row for an address.
// Always ordered by balance descending; no pagination — an account
// typically holds a handful of tokens.
func (r *BalanceRepository) ListByAddress(ctx context.Context, address string) ([]*models.Balance, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT address, token_standard, balance, last_updated_timestamp
		FROM balances WHERE address = $1
		ORDER BY balance DESC`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Balance
	for rows.Next() {
		var b models.Balance
		if err := rows.Scan(&b.Address, &b.TokenStandard, &b.Balance, &b.LastUpdatedTimestamp); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// ListByToken returns balances for a single token, descending by balance.
// Paginated for richlist UIs. Filters out zero balances via the existing
// partial index.
func (r *BalanceRepository) ListByToken(ctx context.Context, tokenStandard string, opts ListOpts) ([]*models.Balance, int64, error) {
	if tokenStandard == "" {
		return nil, 0, fmt.Errorf("token_standard is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT address, token_standard, balance, last_updated_timestamp,
			COUNT(*) OVER () AS total
		FROM balances
		WHERE token_standard = $1 AND balance > 0
		ORDER BY balance DESC
		LIMIT $2 OFFSET $3`,
		tokenStandard, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Balance
		total int64
	)
	for rows.Next() {
		var b models.Balance
		if err := rows.Scan(&b.Address, &b.TokenStandard, &b.Balance, &b.LastUpdatedTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
