package repository

import (
	"context"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RewardRepository struct {
	pool *pgxpool.Pool
}

func NewRewardRepository(pool *pgxpool.Pool) *RewardRepository {
	return &RewardRepository{pool: pool}
}

// UpdateCumulativeRewards updates or inserts cumulative rewards
func (r *RewardRepository) UpdateCumulativeRewards(ctx context.Context, address string, rewardType models.RewardType, amount int64, tokenStandard string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cumulative_rewards (address, reward_type, amount, token_standard)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address, reward_type, token_standard) DO UPDATE SET
			amount = cumulative_rewards.amount + $3`,
		address, int(rewardType), amount, tokenStandard)
	return err
}

// UpdateCumulativeRewardsBatch adds a cumulative rewards update to a batch
func (r *RewardRepository) UpdateCumulativeRewardsBatch(batch *pgx.Batch, address string, rewardType models.RewardType, amount int64, tokenStandard string) {
	batch.Queue(`
		INSERT INTO cumulative_rewards (address, reward_type, amount, token_standard)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address, reward_type, token_standard) DO UPDATE SET
			amount = cumulative_rewards.amount + $3`,
		address, int(rewardType), amount, tokenStandard)
}

// InsertRewardTransaction inserts a reward transaction
func (r *RewardRepository) InsertRewardTransaction(ctx context.Context, rt *models.RewardTransaction) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reward_transactions (hash, address, reward_type, momentum_timestamp,
			momentum_height, account_height, amount, token_standard, source_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (hash) DO NOTHING`,
		rt.Hash, rt.Address, int(rt.RewardType), rt.MomentumTimestamp,
		rt.MomentumHeight, rt.AccountHeight, rt.Amount, rt.TokenStandard, rt.SourceAddress)
	return err
}

// InsertRewardTransactionBatch adds a reward transaction insert to a batch
func (r *RewardRepository) InsertRewardTransactionBatch(batch *pgx.Batch, rt *models.RewardTransaction) {
	batch.Queue(`
		INSERT INTO reward_transactions (hash, address, reward_type, momentum_timestamp,
			momentum_height, account_height, amount, token_standard, source_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (hash) DO NOTHING`,
		rt.Hash, rt.Address, int(rt.RewardType), rt.MomentumTimestamp,
		rt.MomentumHeight, rt.AccountHeight, rt.Amount, rt.TokenStandard, rt.SourceAddress)
}

// GetByAddress retrieves reward transactions for an address
func (r *RewardRepository) GetByAddress(ctx context.Context, address string) ([]*models.RewardTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT hash, address, reward_type, momentum_timestamp, momentum_height,
			account_height, amount, token_standard, source_address
		FROM reward_transactions WHERE address = $1
		ORDER BY momentum_height DESC`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rewards []*models.RewardTransaction
	for rows.Next() {
		var rt models.RewardTransaction
		var rewardType int
		err := rows.Scan(&rt.Hash, &rt.Address, &rewardType, &rt.MomentumTimestamp,
			&rt.MomentumHeight, &rt.AccountHeight, &rt.Amount, &rt.TokenStandard, &rt.SourceAddress)
		if err != nil {
			return nil, err
		}
		rt.RewardType = models.RewardType(rewardType)
		rewards = append(rewards, &rt)
	}
	return rewards, nil
}
