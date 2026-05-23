package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type TokenRepository struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) *TokenRepository {
	return &TokenRepository{pool: pool}
}

// Upsert inserts or updates a token
func (r *TokenRepository) Upsert(ctx context.Context, t *models.Token) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tokens (token_standard, name, symbol, domain, decimals, owner,
			total_supply, max_supply, is_burnable, is_mintable, is_utility)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (token_standard) DO UPDATE SET
			domain = EXCLUDED.domain,
			total_supply = EXCLUDED.total_supply,
			max_supply = EXCLUDED.max_supply`,
		t.TokenStandard, t.Name, t.Symbol, t.Domain, t.Decimals, t.Owner,
		t.TotalSupply, t.MaxSupply, t.IsBurnable, t.IsMintable, t.IsUtility)
	return err
}

// UpsertBatch adds a token upsert to a batch
func (r *TokenRepository) UpsertBatch(batch *pgx.Batch, t *models.Token) {
	batch.Queue(`
		INSERT INTO tokens (token_standard, name, symbol, domain, decimals, owner,
			total_supply, max_supply, is_burnable, is_mintable, is_utility)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (token_standard) DO UPDATE SET
			domain = EXCLUDED.domain,
			total_supply = EXCLUDED.total_supply,
			max_supply = EXCLUDED.max_supply`,
		t.TokenStandard, t.Name, t.Symbol, t.Domain, t.Decimals, t.Owner,
		t.TotalSupply, t.MaxSupply, t.IsBurnable, t.IsMintable, t.IsUtility)
}

// UpdateBurnAmount increments the total burned amount
func (r *TokenRepository) UpdateBurnAmount(ctx context.Context, tokenStandard string, burnAmount int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET total_burned = total_burned + $2
		WHERE token_standard = $1`,
		tokenStandard, burnAmount)
	return err
}

// UpdateBurnAmountBatch adds a burn amount update to a batch
func (r *TokenRepository) UpdateBurnAmountBatch(batch *pgx.Batch, tokenStandard string, burnAmount int64) {
	batch.Queue(`
		UPDATE tokens SET total_burned = total_burned + $2
		WHERE token_standard = $1`,
		tokenStandard, burnAmount)
}

// UpdateLastUpdateTimestamp updates the last update timestamp
func (r *TokenRepository) UpdateLastUpdateTimestamp(ctx context.Context, tokenStandard string, timestamp int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET last_update_timestamp = $2
		WHERE token_standard = $1`,
		tokenStandard, timestamp)
	return err
}

// UpdateLastUpdateTimestampBatch adds a timestamp update to a batch
func (r *TokenRepository) UpdateLastUpdateTimestampBatch(batch *pgx.Batch, tokenStandard string, timestamp int64) {
	batch.Queue(`
		UPDATE tokens SET last_update_timestamp = $2
		WHERE token_standard = $1`,
		tokenStandard, timestamp)
}

// IncrementTransactionCount increments the transaction count
func (r *TokenRepository) IncrementTransactionCount(ctx context.Context, tokenStandard string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET transaction_count = transaction_count + 1
		WHERE token_standard = $1`,
		tokenStandard)
	return err
}

// IncrementTransactionCountBatch adds a transaction count increment to a batch
func (r *TokenRepository) IncrementTransactionCountBatch(batch *pgx.Batch, tokenStandard string) {
	batch.Queue(`
		UPDATE tokens SET transaction_count = transaction_count + 1
		WHERE token_standard = $1`,
		tokenStandard)
}

// UpdateHolderCount updates the holder count
func (r *TokenRepository) UpdateHolderCount(ctx context.Context, tokenStandard string, count int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tokens SET holder_count = $2
		WHERE token_standard = $1`,
		tokenStandard, count)
	return err
}

// GetByStandard returns a token by its ZTS identifier. Returns pgx.ErrNoRows
// if not present.
func (r *TokenRepository) GetByStandard(ctx context.Context, tokenStandard string) (*models.Token, error) {
	if tokenStandard == "" {
		return nil, fmt.Errorf("token_standard is required")
	}
	var t models.Token
	err := r.pool.QueryRow(ctx, `
		SELECT token_standard, name, symbol, domain, decimals, owner,
			total_supply, max_supply, is_burnable, is_mintable, is_utility,
			total_burned, last_update_timestamp, holder_count, transaction_count
		FROM tokens WHERE token_standard = $1`, tokenStandard).Scan(
		&t.TokenStandard, &t.Name, &t.Symbol, &t.Domain, &t.Decimals, &t.Owner,
		&t.TotalSupply, &t.MaxSupply, &t.IsBurnable, &t.IsMintable, &t.IsUtility,
		&t.TotalBurned, &t.LastUpdateTimestamp, &t.HolderCount, &t.TransactionCount)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns tokens ordered by holder_count descending (most-held first)
// with the total row count. Pagination friendly for large catalogs.
func (r *TokenRepository) List(ctx context.Context, opts ListOpts) ([]*models.Token, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT token_standard, name, symbol, domain, decimals, owner,
			total_supply, max_supply, is_burnable, is_mintable, is_utility,
			total_burned, last_update_timestamp, holder_count, transaction_count,
			COUNT(*) OVER () AS total
		FROM tokens
		ORDER BY holder_count DESC, token_standard ASC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Token
		total int64
	)
	for rows.Next() {
		var t models.Token
		if err := rows.Scan(&t.TokenStandard, &t.Name, &t.Symbol, &t.Domain, &t.Decimals, &t.Owner,
			&t.TotalSupply, &t.MaxSupply, &t.IsBurnable, &t.IsMintable, &t.IsUtility,
			&t.TotalBurned, &t.LastUpdateTimestamp, &t.HolderCount, &t.TransactionCount, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM tokens`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// GetAll retrieves all tokens
func (r *TokenRepository) GetAll(ctx context.Context) ([]*models.Token, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT token_standard, name, symbol, domain, decimals, owner,
			total_supply, max_supply, is_burnable, is_mintable, is_utility,
			total_burned, last_update_timestamp, holder_count, transaction_count
		FROM tokens`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.Token
	for rows.Next() {
		var t models.Token
		err := rows.Scan(&t.TokenStandard, &t.Name, &t.Symbol, &t.Domain, &t.Decimals, &t.Owner,
			&t.TotalSupply, &t.MaxSupply, &t.IsBurnable, &t.IsMintable, &t.IsUtility,
			&t.TotalBurned, &t.LastUpdateTimestamp, &t.HolderCount, &t.TransactionCount)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, &t)
	}
	return tokens, nil
}
