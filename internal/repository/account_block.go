package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type AccountBlockRepository struct {
	pool *pgxpool.Pool
}

func NewAccountBlockRepository(pool *pgxpool.Pool) *AccountBlockRepository {
	return &AccountBlockRepository{pool: pool}
}

// sanitizeJSONForPostgres removes null bytes and other invalid Unicode escape sequences
// that PostgreSQL JSONB doesn't support. PostgreSQL rejects \u0000 in JSONB.
func sanitizeJSONForPostgres(s string) string {
	// Remove literal \u0000 escape sequences (as they appear in JSON)
	s = strings.ReplaceAll(s, `\u0000`, "")
	// Also remove actual null bytes (shouldn't appear in JSON, but just in case)
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// Insert inserts an account block
func (r *AccountBlockRepository) Insert(ctx context.Context, ab *models.AccountBlock, txData *models.TxData) error {
	input := "{}"
	if txData != nil && len(txData.Inputs) > 0 {
		inputBytes, err := json.Marshal(txData.Inputs)
		if err == nil {
			input = sanitizeJSONForPostgres(string(inputBytes))
		}
		// On marshal error, keep default "{}"
	}
	method := ""
	if txData != nil {
		method = txData.Method
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO account_blocks (hash, momentum_hash, momentum_timestamp, momentum_height, block_type,
			height, address, to_address, amount, token_standard, data, method, input, paired_account_block)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (hash) DO UPDATE SET
			method = EXCLUDED.method,
			input = EXCLUDED.input,
			paired_account_block = EXCLUDED.paired_account_block`,
		ab.Hash, ab.MomentumHash, ab.MomentumTimestamp, ab.MomentumHeight, ab.BlockType,
		ab.Height, ab.Address, ab.ToAddress, ab.Amount, ab.TokenStandard, ab.Data, method, input, ab.PairedAccountBlock)
	return err
}

// InsertBatch adds an account block insert to a batch
func (r *AccountBlockRepository) InsertBatch(batch *pgx.Batch, ab *models.AccountBlock, txData *models.TxData) {
	input := "{}"
	if txData != nil && len(txData.Inputs) > 0 {
		inputBytes, err := json.Marshal(txData.Inputs)
		if err == nil {
			input = sanitizeJSONForPostgres(string(inputBytes))
		}
		// On marshal error, keep default "{}"
	}
	method := ""
	if txData != nil {
		method = txData.Method
	}

	batch.Queue(`
		INSERT INTO account_blocks (hash, momentum_hash, momentum_timestamp, momentum_height, block_type,
			height, address, to_address, amount, token_standard, data, method, input, paired_account_block)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (hash) DO UPDATE SET
			method = EXCLUDED.method,
			input = EXCLUDED.input,
			paired_account_block = EXCLUDED.paired_account_block`,
		ab.Hash, ab.MomentumHash, ab.MomentumTimestamp, ab.MomentumHeight, ab.BlockType,
		ab.Height, ab.Address, ab.ToAddress, ab.Amount, ab.TokenStandard, ab.Data, method, input, ab.PairedAccountBlock)
}

// UpdatePairedBlock updates the paired account block reference
func (r *AccountBlockRepository) UpdatePairedBlock(ctx context.Context, hash, pairedHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE account_blocks SET paired_account_block = $2 WHERE hash = $1`,
		hash, pairedHash)
	return err
}

// UpdatePairedBlockBatch adds a paired block update to a batch
func (r *AccountBlockRepository) UpdatePairedBlockBatch(batch *pgx.Batch, hash, pairedHash string) {
	batch.Queue(`
		UPDATE account_blocks SET paired_account_block = $2 WHERE hash = $1`,
		hash, pairedHash)
}

// UpdateDescendantOf updates the descendant_of field
func (r *AccountBlockRepository) UpdateDescendantOf(ctx context.Context, hash, parentHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE account_blocks SET descendant_of = $2 WHERE hash = $1`,
		hash, parentHash)
	return err
}

// UpdateDescendantOfBatch adds a descendant_of update to a batch
func (r *AccountBlockRepository) UpdateDescendantOfBatch(batch *pgx.Batch, hash, parentHash string) {
	batch.Queue(`
		UPDATE account_blocks SET descendant_of = $2 WHERE hash = $1`,
		hash, parentHash)
}

// GetByHash retrieves an account block by hash
func (r *AccountBlockRepository) GetByHash(ctx context.Context, hash string) (*models.AccountBlock, error) {
	var ab models.AccountBlock
	err := r.pool.QueryRow(ctx, `
		SELECT hash, momentum_hash, momentum_timestamp, momentum_height, block_type,
			height, address, to_address, amount, token_standard, data, method, input,
			paired_account_block, descendant_of
		FROM account_blocks WHERE hash = $1`, hash).Scan(
		&ab.Hash, &ab.MomentumHash, &ab.MomentumTimestamp, &ab.MomentumHeight, &ab.BlockType,
		&ab.Height, &ab.Address, &ab.ToAddress, &ab.Amount, &ab.TokenStandard, &ab.Data,
		&ab.Method, &ab.Input, &ab.PairedAccountBlock, &ab.DescendantOf)
	if err != nil {
		return nil, err
	}
	return &ab, nil
}

// GetRewardDetails retrieves reward details for a receive block
func (r *AccountBlockRepository) GetRewardDetails(ctx context.Context, receiveBlockHash string, rewardContracts []string) (map[string]interface{}, error) {
	var rewardAmount int64
	var source, tokenStandard string

	err := r.pool.QueryRow(ctx, `
		SELECT T1.amount as reward_amount, T2.address as source, T1.token_standard
		FROM account_blocks T1
		INNER JOIN account_blocks T2
			ON T1.descendant_of = T2.paired_account_block AND T2.method = 'Mint'
		INNER JOIN account_blocks T3
			ON T2.descendant_of = T3.paired_account_block AND T3.method = 'CollectReward'
		WHERE T1.hash = $1
			AND (T1.token_standard = $2 OR T1.token_standard = $3)
			AND T2.address = ANY($4)
		ORDER BY T1.momentum_height DESC LIMIT 1`,
		receiveBlockHash, models.ZnnTokenStandard, models.QsrTokenStandard, rewardContracts).Scan(
		&rewardAmount, &source, &tokenStandard)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"rewardAmount":  rewardAmount,
		"source":        source,
		"tokenStandard": tokenStandard,
	}, nil
}

// accountBlockCols is the SELECT-list (plus COUNT(*) OVER ()) used by every
// List method. Kept in one place so column order stays in sync with Scan.
const accountBlockCols = `hash, momentum_hash, momentum_timestamp, momentum_height, block_type,
	height, address, to_address, amount, token_standard, data, method, input,
	paired_account_block, descendant_of`

// scanAccountBlock reads one account_blocks row. total may be nil for
// callers that compute total via a separate query (see List below); when
// non-nil, the SELECT must include `COUNT(*) OVER () AS total` as the
// final column.
func scanAccountBlock(rows pgx.Row, ab *models.AccountBlock, total *int64) error {
	dst := []interface{}{
		&ab.Hash, &ab.MomentumHash, &ab.MomentumTimestamp, &ab.MomentumHeight, &ab.BlockType,
		&ab.Height, &ab.Address, &ab.ToAddress, &ab.Amount, &ab.TokenStandard, &ab.Data,
		&ab.Method, &ab.Input, &ab.PairedAccountBlock, &ab.DescendantOf,
	}
	if total != nil {
		dst = append(dst, total)
	}
	return rows.Scan(dst...)
}

// List returns account blocks ordered by momentum_height (newest-first
// by default), with an approximate total row count for pagination.
//
// Two queries by design. The original pattern inlined COUNT(*) OVER ()
// which forced a full sequential scan of the ~3M-row account_blocks
// table even on page 1 — production requests blew past the API's 30s
// WriteTimeout. The page query now reads only LIMIT rows via the
// momentum_height index; the total comes from pg_class.reltuples,
// which Postgres maintains via ANALYZE and autovacuum.
//
// reltuples is approximate: it lags behind autovacuum by however long
// the table has been growing since the last stats update. The
// scoped variant ListByAddress keeps the exact COUNT(*) OVER ()
// because its WHERE clause shrinks the scan to per-address row counts
// (typically far under a thousand) where the exact count is cheap.
func (r *AccountBlockRepository) List(ctx context.Context, opts ListOpts) ([]*models.AccountBlock, int64, error) {
	query := fmt.Sprintf(`
		SELECT `+accountBlockCols+`
		FROM account_blocks
		ORDER BY momentum_height %s
		LIMIT $1 OFFSET $2`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.AccountBlock
	for rows.Next() {
		var ab models.AccountBlock
		if err := scanAccountBlock(rows, &ab, nil); err != nil {
			return nil, 0, err
		}
		out = append(out, &ab)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT GREATEST(reltuples, 0)::BIGINT
		 FROM pg_class WHERE relname = 'account_blocks'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListByAddress returns account blocks where the address is either sender
// or recipient. Useful for "transactions for an account" queries.
func (r *AccountBlockRepository) ListByAddress(ctx context.Context, address string, opts ListOpts) ([]*models.AccountBlock, int64, error) {
	if address == "" {
		return nil, 0, errors.New("address is required")
	}
	query := fmt.Sprintf(`
		SELECT `+accountBlockCols+`, COUNT(*) OVER () AS total
		FROM account_blocks
		WHERE address = $1 OR to_address = $1
		ORDER BY momentum_height %s
		LIMIT $2 OFFSET $3`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, address, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.AccountBlock
		total int64
	)
	for rows.Next() {
		var ab models.AccountBlock
		if err := scanAccountBlock(rows, &ab, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &ab)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool,
			`SELECT COUNT(*) FROM account_blocks WHERE address = $1 OR to_address = $1`, address)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
