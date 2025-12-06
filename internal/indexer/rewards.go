package indexer

import (
	"context"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"
)

// indexLiquidityReward handles liquidity reward receive transactions
func (i *Indexer) indexLiquidityReward(batch *pgx.Batch, block *api.AccountBlock, m *api.Momentum) {
	if block.PairedAccountBlock == nil {
		return
	}

	rt := &models.RewardTransaction{
		Hash:              block.Hash.String(),
		Address:           block.Address.String(),
		RewardType:        models.RewardTypeLiquidity,
		MomentumTimestamp: int64(m.TimestampUnix),
		MomentumHeight:    int64(m.Height),
		AccountHeight:     int64(block.Height),
		Amount:            block.PairedAccountBlock.Amount.Int64(),
		TokenStandard:     block.PairedAccountBlock.TokenStandard.String(),
		SourceAddress:     block.PairedAccountBlock.Address.String(),
	}

	i.repos.Reward.InsertRewardTransactionBatch(batch, rt)
	i.repos.Reward.UpdateCumulativeRewardsBatch(batch, rt.Address, rt.RewardType, rt.Amount, rt.TokenStandard)

	i.logger.Debug("indexed liquidity reward",
		zap.String("address", rt.Address),
		zap.Int64("amount", rt.Amount))
}

// indexReceivedReward handles received reward transactions from embedded contracts
func (i *Indexer) indexReceivedReward(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, m *api.Momentum) {
	if block.PairedAccountBlock == nil {
		return
	}

	sourceAddress := block.PairedAccountBlock.Address.String()
	rewardType := i.determineRewardType(sourceAddress)

	if rewardType == models.RewardTypeUnknown {
		return
	}

	rt := &models.RewardTransaction{
		Hash:              block.Hash.String(),
		Address:           block.Address.String(),
		RewardType:        rewardType,
		MomentumTimestamp: int64(m.TimestampUnix),
		MomentumHeight:    int64(m.Height),
		AccountHeight:     int64(block.Height),
		Amount:            block.PairedAccountBlock.Amount.Int64(),
		TokenStandard:     block.PairedAccountBlock.TokenStandard.String(),
		SourceAddress:     sourceAddress,
	}

	i.repos.Reward.InsertRewardTransactionBatch(batch, rt)
	i.repos.Reward.UpdateCumulativeRewardsBatch(batch, rt.Address, rt.RewardType, rt.Amount, rt.TokenStandard)

	i.logger.Debug("indexed reward",
		zap.String("type", rt.RewardType.String()),
		zap.String("address", rt.Address),
		zap.Int64("amount", rt.Amount))
}

// determineRewardType determines the reward type based on source address
func (i *Indexer) determineRewardType(sourceAddress string) models.RewardType {
	switch sourceAddress {
	case models.PillarAddress:
		return models.RewardTypePillar
	case models.SentinelAddress:
		return models.RewardTypeSentinel
	case models.StakeAddress:
		return models.RewardTypeStake
	case models.LiquidityAddress:
		return models.RewardTypeLiquidity
	default:
		return models.RewardTypeUnknown
	}
}
