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

// accountBlockCols is the shared SELECT-list for account_block reads. Kept in
// one place so column order stays in sync with Scan.
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
// the table has been growing since the last stats update. The address
// scoped variant ListByAddress keeps an exact total by reading the
// cached accounts.tx_count counter instead of counting account_blocks
// at request time.
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
//
// pagination.total is sourced from the cached accounts.tx_count (a
// per-account counter incremented in the same transaction as each
// block insert, see repository.AccountRepository.BumpTxCountBatch).
// By construction it equals COUNT(*) WHERE address = $1 OR to_address
// = $1, but is an O(1) PK lookup rather than a full address scan —
// production accounts have hundreds of thousands of rows, and the
// inline COUNT(*) OVER () we used to compute here took 30s+ wall time
// on every page load regardless of page/page_size. Missing account
// row means "address never observed", so total = 0.
func (r *AccountBlockRepository) ListByAddress(ctx context.Context, address string, opts ListOpts) ([]*models.AccountBlock, int64, error) {
	if address == "" {
		return nil, 0, errors.New("address is required")
	}
	query := fmt.Sprintf(`
		SELECT `+accountBlockCols+`
		FROM account_blocks
		WHERE address = $1 OR to_address = $1
		ORDER BY momentum_height %s
		LIMIT $2 OFFSET $3`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, address, opts.Limit, opts.Offset)
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
		`SELECT tx_count FROM accounts WHERE address = $1`, address).Scan(&total); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, err
		}
		total = 0
	}
	return out, total, nil
}

// ListByMomentumHeightRange returns account_blocks whose momentum_height
// is in [fromMomentumHeight, toMomentumHeight] inclusive, ordered first
// by momentum_height ASC then by hash (deterministic tiebreak). Powers
// the transactions WS stream's replay/catch-up path — a single
// indexed range scan, optionally narrowed by sender/recipient address.
//
// addressFilter == "" disables the address WHERE — used by clients
// streaming without ?address=. Otherwise the WHERE matches either
// sender (address) or recipient (to_address), mirroring the REST
// ListByAddress endpoint.
//
// The caller must pass a sensible limit (the stream handler uses
// streamReplayMaxRows = 10,000). limit == 0 means no cap; callers
// should not rely on that.
func (r *AccountBlockRepository) ListByMomentumHeightRange(
	ctx context.Context,
	fromMomentumHeight, toMomentumHeight int64,
	addressFilter string,
	limit int,
) ([]*models.AccountBlock, error) {
	if fromMomentumHeight > toMomentumHeight {
		return nil, nil
	}
	args := []interface{}{fromMomentumHeight, toMomentumHeight}
	where := `WHERE momentum_height >= $1 AND momentum_height <= $2`
	if addressFilter != "" {
		args = append(args, addressFilter)
		where += ` AND (address = $3 OR to_address = $3)`
	}
	q := `
		SELECT ` + accountBlockCols + `
		FROM account_blocks
		` + where + `
		ORDER BY momentum_height ASC, hash ASC`
	if limit > 0 {
		args = append(args, limit)
		q += fmt.Sprintf(` LIMIT $%d`, len(args))
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.AccountBlock
	for rows.Next() {
		var ab models.AccountBlock
		if err := scanAccountBlock(rows, &ab, nil); err != nil {
			return nil, err
		}
		out = append(out, &ab)
	}
	return out, rows.Err()
}
