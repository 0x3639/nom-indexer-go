package repository

import (
	"context"
	"encoding/json"

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

// Insert inserts an account block
func (r *AccountBlockRepository) Insert(ctx context.Context, ab *models.AccountBlock, txData *models.TxData) error {
	input := "{}"
	if txData != nil && len(txData.Inputs) > 0 {
		inputBytes, err := json.Marshal(txData.Inputs)
		if err == nil {
			input = string(inputBytes)
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
			input = string(inputBytes)
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
