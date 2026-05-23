package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// StatHistoryRepository owns the four *_stat_histories tables. The cron loop
// computes a day's row and upserts on (date, key) so re-running mid-day is
// safe and idempotent.
type StatHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewStatHistoryRepository(pool *pgxpool.Pool) *StatHistoryRepository {
	return &StatHistoryRepository{pool: pool}
}

func (r *StatHistoryRepository) UpsertNetworkStat(ctx context.Context, s *models.NetworkStatHistory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO network_stat_histories (date, total_tx, daily_tx, total_addresses,
			daily_addresses, active_addresses, total_tokens, daily_tokens,
			total_stakes, daily_stakes, total_fusions, daily_fusions,
			total_pillars, total_sentinels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (date) DO UPDATE SET
			total_tx = EXCLUDED.total_tx,
			daily_tx = EXCLUDED.daily_tx,
			total_addresses = EXCLUDED.total_addresses,
			daily_addresses = EXCLUDED.daily_addresses,
			active_addresses = EXCLUDED.active_addresses,
			total_tokens = EXCLUDED.total_tokens,
			daily_tokens = EXCLUDED.daily_tokens,
			total_stakes = EXCLUDED.total_stakes,
			daily_stakes = EXCLUDED.daily_stakes,
			total_fusions = EXCLUDED.total_fusions,
			daily_fusions = EXCLUDED.daily_fusions,
			total_pillars = EXCLUDED.total_pillars,
			total_sentinels = EXCLUDED.total_sentinels`,
		s.Date, s.TotalTx, s.DailyTx, s.TotalAddresses,
		s.DailyAddresses, s.ActiveAddresses, s.TotalTokens, s.DailyTokens,
		s.TotalStakes, s.DailyStakes, s.TotalFusions, s.DailyFusions,
		s.TotalPillars, s.TotalSentinels)
	return err
}

func (r *StatHistoryRepository) UpsertTokenStat(ctx context.Context, s *models.TokenStatHistory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO token_stat_histories (date, token_standard, daily_minted, daily_burned,
			total_supply, total_holders, total_transactions)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (date, token_standard) DO UPDATE SET
			daily_minted = EXCLUDED.daily_minted,
			daily_burned = EXCLUDED.daily_burned,
			total_supply = EXCLUDED.total_supply,
			total_holders = EXCLUDED.total_holders,
			total_transactions = EXCLUDED.total_transactions`,
		s.Date, s.TokenStandard, s.DailyMinted, s.DailyBurned,
		s.TotalSupply, s.TotalHolders, s.TotalTransactions)
	return err
}

func (r *StatHistoryRepository) UpsertPillarStat(ctx context.Context, s *models.PillarStatHistory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pillar_stat_histories (date, pillar_owner_address, rank, weight,
			momentum_rewards, delegate_rewards, total_delegators)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (date, pillar_owner_address) DO UPDATE SET
			rank = EXCLUDED.rank,
			weight = EXCLUDED.weight,
			momentum_rewards = EXCLUDED.momentum_rewards,
			delegate_rewards = EXCLUDED.delegate_rewards,
			total_delegators = EXCLUDED.total_delegators`,
		s.Date, s.PillarOwnerAddress, s.Rank, s.Weight,
		s.MomentumRewards, s.DelegateRewards, s.TotalDelegators)
	return err
}

func (r *StatHistoryRepository) UpsertBridgeStat(ctx context.Context, s *models.BridgeStatHistory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_stat_histories (date, network_class, chain_id, token_standard,
			wrap_tx_count, wrapped_amount, unwrap_tx_count, unwrapped_amount, total_volume)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (date, network_class, chain_id, token_standard) DO UPDATE SET
			wrap_tx_count = EXCLUDED.wrap_tx_count,
			wrapped_amount = EXCLUDED.wrapped_amount,
			unwrap_tx_count = EXCLUDED.unwrap_tx_count,
			unwrapped_amount = EXCLUDED.unwrapped_amount,
			total_volume = EXCLUDED.total_volume`,
		s.Date, s.NetworkClass, s.ChainID, s.TokenStandard,
		s.WrapTxCount, s.WrappedAmount, s.UnwrapTxCount, s.UnwrappedAmount, s.TotalVolume)
	return err
}
