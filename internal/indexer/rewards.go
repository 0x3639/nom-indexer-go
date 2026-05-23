package indexer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
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
	receiverAddress := block.Address.String()
	rewardType := i.classifyReward(ctx, sourceAddress, receiverAddress)

	if rewardType == models.RewardTypeUnknown {
		return
	}

	rt := &models.RewardTransaction{
		Hash:              block.Hash.String(),
		Address:           receiverAddress,
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

// classifyReward maps (source contract, receiver) to a RewardType. Pillar
// rewards split into PillarTypePillar (when the receiver is the pillar's own
// withdraw address) vs RewardTypeDelegation (delegator share). Other sources
// route directly to their type. Errors fall back to "Pillar" so we don't lose
// the row entirely when the lookup fails — that matches the historical
// behavior of determineRewardType.
func (i *Indexer) classifyReward(ctx context.Context, sourceAddress, receiverAddress string) models.RewardType {
	switch sourceAddress {
	case models.SentinelAddress:
		return models.RewardTypeSentinel
	case models.StakeAddress:
		return models.RewardTypeStake
	case models.LiquidityAddress:
		return models.RewardTypeLiquidity
	case models.PillarAddress:
		isPillar, err := i.repos.Pillar.IsWithdrawAddress(ctx, receiverAddress)
		if err != nil {
			i.logger.Warn("classifyReward: IsWithdrawAddress failed, defaulting to Pillar",
				zap.String("receiver", receiverAddress),
				zap.Error(err))
			return models.RewardTypePillar
		}
		if isPillar {
			return models.RewardTypePillar
		}
		return models.RewardTypeDelegation
	default:
		return models.RewardTypeUnknown
	}
}

// determineRewardType is retained for backwards compatibility (used by tests
// and by the historical-data backfill script). It does not split Pillar vs
// Delegation — callers that need the split should use classifyReward.
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
