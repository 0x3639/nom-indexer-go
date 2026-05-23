package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
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

// CumulativeByAddress returns the rolled-up cumulative_rewards rows for
// an address (one row per reward_type x token_standard).
func (r *RewardRepository) CumulativeByAddress(ctx context.Context, address string) ([]*models.CumulativeReward, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, address, reward_type, amount, token_standard
		FROM cumulative_rewards WHERE address = $1
		ORDER BY reward_type ASC, token_standard ASC`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.CumulativeReward
	for rows.Next() {
		var c models.CumulativeReward
		var rt int
		if err := rows.Scan(&c.ID, &c.Address, &rt, &c.Amount, &c.TokenStandard); err != nil {
			return nil, err
		}
		c.RewardType = models.RewardType(rt)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// HistoryByAddress returns per-event reward transactions for an address,
// newest first, paginated.
func (r *RewardRepository) HistoryByAddress(ctx context.Context, address string, opts ListOpts) ([]*models.RewardTransaction, int64, error) {
	if address == "" {
		return nil, 0, fmt.Errorf("address is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT hash, address, reward_type, momentum_timestamp, momentum_height,
			account_height, amount, token_standard, source_address,
			COUNT(*) OVER () AS total
		FROM reward_transactions
		WHERE address = $1
		ORDER BY momentum_height DESC
		LIMIT $2 OFFSET $3`, address, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.RewardTransaction
		total int64
	)
	for rows.Next() {
		var rt models.RewardTransaction
		var rtype int
		if err := rows.Scan(&rt.Hash, &rt.Address, &rtype, &rt.MomentumTimestamp,
			&rt.MomentumHeight, &rt.AccountHeight, &rt.Amount, &rt.TokenStandard, &rt.SourceAddress, &total); err != nil {
			return nil, 0, err
		}
		rt.RewardType = models.RewardType(rtype)
		out = append(out, &rt)
	}
	return out, total, rows.Err()
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
