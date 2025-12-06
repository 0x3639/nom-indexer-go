package indexer

import (
	"context"
	"strconv"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"
)

// indexEmbeddedContracts handles indexing for embedded contract interactions
func (i *Indexer) indexEmbeddedContracts(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	if txData == nil {
		return
	}

	address := block.Address.String()

	switch address {
	case models.PillarAddress:
		i.indexPillarContract(ctx, batch, block, txData, m)
	case models.StakeAddress:
		i.indexStakeContract(ctx, batch, block, txData, m)
	case models.SentinelAddress:
		i.indexSentinelContract(ctx, batch, block, txData, m)
	case models.PlasmaAddress:
		i.indexPlasmaContract(ctx, batch, block, txData, m)
	case models.AcceleratorAddress:
		i.indexAcceleratorContract(ctx, batch, block, txData, m)
	case models.TokenAddress:
		i.indexTokenContract(ctx, batch, block, txData, m)
	}
}

// indexPillarContract handles pillar contract events
func (i *Indexer) indexPillarContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "Register", "RegisterLegacy":
		// Record pillar update
		name := txData.Inputs["name"]
		producerAddress := txData.Inputs["producerAddress"]
		rewardAddress := txData.Inputs["rewardAddress"]
		if name != "" && block.PairedAccountBlock != nil {
			ownerAddress := block.PairedAccountBlock.Address.String()
			update := &models.PillarUpdate{
				OwnerAddress:      ownerAddress,
				ProducerAddress:   producerAddress,
				WithdrawAddress:   rewardAddress,
				Name:              name,
				MomentumHeight:    int64(m.Height),
				MomentumTimestamp: int64(m.TimestampUnix),
				MomentumHash:      m.Hash.String(),
			}
			i.repos.PillarUpdate.InsertBatch(batch, update)

			// Check descendant blocks for Burn transaction to get slot cost
			// Note: DescendantBlocks are nom.AccountBlock type with limited fields
			// The descendant's ToAddress and Amount are accessible for determining QSR burn
			if len(block.DescendantBlocks) > 0 {
				descendant := block.DescendantBlocks[0]
				if descendant.ToAddress.String() == models.TokenAddress {
					// The descendant is a Burn transaction to the token contract
					slotCostQsr := descendant.Amount.Int64()
					i.repos.Pillar.UpdateSpawnInfoBatch(batch, ownerAddress, int64(m.TimestampUnix), slotCostQsr)
					i.logger.Debug("pillar registered with spawn info",
						zap.String("name", name),
						zap.String("owner", ownerAddress),
						zap.Int64("slotCost", slotCostQsr))
				}
			}
		}
	case "UpdatePillar":
		// Record pillar update
		name := txData.Inputs["name"]
		producerAddress := txData.Inputs["producerAddress"]
		rewardAddress := txData.Inputs["rewardAddress"]
		pillarOwner := txData.Inputs["pillarOwner"]
		if name != "" && pillarOwner != "" {
			update := &models.PillarUpdate{
				OwnerAddress:      pillarOwner,
				ProducerAddress:   producerAddress,
				WithdrawAddress:   rewardAddress,
				Name:              name,
				MomentumHeight:    int64(m.Height),
				MomentumTimestamp: int64(m.TimestampUnix),
				MomentumHash:      m.Hash.String(),
			}
			i.repos.PillarUpdate.InsertBatch(batch, update)
		}
	case "Delegate":
		// Update account delegation
		pillarName := txData.Inputs["name"]
		if pillarName != "" && block.PairedAccountBlock != nil {
			pillarOwner := i.getPillarOwnerAddress(pillarName)
			if pillarOwner != "" {
				delegatorAddress := block.PairedAccountBlock.Address.String()
				i.repos.Account.UpdateDelegateBatch(batch, delegatorAddress, pillarOwner, int64(m.TimestampUnix))
				i.logger.Debug("delegation recorded",
					zap.String("delegator", delegatorAddress),
					zap.String("pillar", pillarName))
			}
		}
	case "Undelegate":
		// Clear account delegation
		if block.PairedAccountBlock != nil {
			delegatorAddress := block.PairedAccountBlock.Address.String()
			i.repos.Account.UpdateDelegateBatch(batch, delegatorAddress, "", 0)
			i.logger.Debug("undelegation recorded", zap.String("delegator", delegatorAddress))
		}
	case "Revoke":
		// Mark pillar as revoked
		pillarName := txData.Inputs["name"]
		if pillarName != "" && block.PairedAccountBlock != nil {
			pillarOwner := block.PairedAccountBlock.Address.String()
			i.repos.Pillar.SetAsRevokedBatch(batch, pillarOwner, pillarName, int64(m.TimestampUnix))
			i.logger.Debug("pillar revoked",
				zap.String("name", pillarName),
				zap.String("owner", pillarOwner))
		}
	}
}

// indexStakeContract handles stake contract events
func (i *Indexer) indexStakeContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "Stake":
		if block.PairedAccountBlock != nil {
			durationStr := txData.Inputs["durationInSec"]
			duration, err := strconv.Atoi(durationStr)
			if err != nil {
				i.logger.Warn("invalid stake duration", zap.String("duration", durationStr), zap.Error(err))
				duration = 0
			}

			stakeID := block.PairedAccountBlock.Hash.String()
			stake := &models.Stake{
				ID:                  stakeID,
				Address:             block.PairedAccountBlock.Address.String(),
				ZnnAmount:           block.PairedAccountBlock.Amount.Int64(),
				StartTimestamp:      int64(m.TimestampUnix),
				DurationInSec:       duration,
				ExpirationTimestamp: int64(m.TimestampUnix) + int64(duration),
				IsActive:            true,
				CancelID:            i.getStakeCancelID(stakeID),
			}
			i.repos.Stake.InsertBatch(batch, stake)
		}
	case "Cancel":
		stakeID := txData.Inputs["id"]
		if stakeID != "" && block.PairedAccountBlock != nil {
			// Compute the cancel ID from the stake ID and mark the stake as inactive
			cancelID := i.getStakeCancelID(stakeID)
			address := block.PairedAccountBlock.Address.String()
			i.repos.Stake.SetInactiveBatch(batch, cancelID, address)
		}
	}
}

// indexSentinelContract handles sentinel contract events
func (i *Indexer) indexSentinelContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "Revoke":
		// Mark sentinel as inactive
		if block.PairedAccountBlock != nil {
			owner := block.PairedAccountBlock.Address.String()
			i.repos.Sentinel.SetInactiveBatch(batch, owner)
			i.logger.Debug("sentinel revoked", zap.String("owner", owner))
		}
	default:
		i.logger.Debug("sentinel contract event", zap.String("method", method))
	}
}

// indexPlasmaContract handles plasma contract events
func (i *Indexer) indexPlasmaContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "Fuse":
		if block.PairedAccountBlock != nil {
			beneficiary := txData.Inputs["address"]
			if beneficiary == "" {
				beneficiary = block.PairedAccountBlock.Address.String()
			}
			fusionID := block.PairedAccountBlock.Hash.String()
			fusion := &models.Fusion{
				ID:                fusionID,
				Address:           block.PairedAccountBlock.Address.String(),
				Beneficiary:       beneficiary,
				QsrAmount:         block.PairedAccountBlock.Amount.Int64(),
				MomentumTimestamp: int64(m.TimestampUnix),
				MomentumHeight:    int64(m.Height),
				MomentumHash:      m.Hash.String(),
				ExpirationHeight:  int64(m.Height) + models.FusionExpirationTime/10, // Approximate blocks
				IsActive:          true,
				CancelID:          i.getFusionCancelID(fusionID),
			}
			i.repos.Fusion.InsertBatch(batch, fusion)
		}
	case "CancelFuse":
		fusionID := txData.Inputs["id"]
		if fusionID != "" && block.PairedAccountBlock != nil {
			// Compute the cancel ID from the fusion ID and mark the fusion as inactive
			cancelID := i.getFusionCancelID(fusionID)
			address := block.PairedAccountBlock.Address.String()
			i.repos.Fusion.SetInactiveBatch(batch, cancelID, address)
		}
	}
}

// indexAcceleratorContract handles accelerator contract events
func (i *Indexer) indexAcceleratorContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "VoteByName", "VoteByProdAddress":
		votingID := txData.Inputs["id"]
		voteValueStr := txData.Inputs["vote"]
		voteValue, err := strconv.Atoi(voteValueStr)
		if err != nil {
			i.logger.Warn("invalid vote value", zap.String("vote", voteValueStr), zap.Error(err))
			voteValue = 0
		}

		if votingID != "" && block.PairedAccountBlock != nil {
			// Resolve project and phase IDs from voting ID
			var projectID, phaseID string

			// First try to find if this is a project vote
			projectID, err := i.repos.Project.GetIDFromVotingID(ctx, votingID)
			if err != nil || projectID == "" {
				// Not a project, try to find if it's a phase vote
				projectID, phaseID, _ = i.repos.ProjectPhase.GetProjectAndPhaseIDFromVotingID(ctx, votingID)
			}

			// Get voter address - for VoteByName, resolve pillar name to owner
			voterAddress := block.PairedAccountBlock.Address.String()
			if method == "VoteByName" {
				pillarName := txData.Inputs["name"]
				if pillarName != "" {
					if owner := i.getPillarOwnerAddress(pillarName); owner != "" {
						voterAddress = owner
					}
				}
			}

			vote := &models.Vote{
				VotingID:          votingID,
				VoterAddress:      voterAddress,
				ProjectID:         projectID,
				PhaseID:           phaseID,
				Vote:              int16(voteValue),
				MomentumTimestamp: int64(m.TimestampUnix),
				MomentumHeight:    int64(m.Height),
				MomentumHash:      m.Hash.String(),
			}
			i.repos.Vote.InsertBatch(batch, vote)

			i.logger.Debug("vote recorded",
				zap.String("votingID", votingID),
				zap.String("projectID", projectID),
				zap.String("phaseID", phaseID),
				zap.String("voter", voterAddress))
		}
	case "CreateProject":
		i.logger.Debug("project created", zap.String("method", method))
	case "AddPhase", "UpdatePhase":
		i.logger.Debug("phase updated", zap.String("method", method))
	}
}

// indexTokenContract handles token contract events
func (i *Indexer) indexTokenContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	method := txData.Method

	switch method {
	case "Burn":
		// Update token burn amount
		if block.PairedAccountBlock != nil {
			tokenStandard := block.PairedAccountBlock.TokenStandard.String()
			burnAmount := block.PairedAccountBlock.Amount.Int64()
			i.repos.Token.UpdateBurnAmountBatch(batch, tokenStandard, burnAmount)
			i.logger.Debug("token burn recorded",
				zap.String("token", tokenStandard),
				zap.Int64("amount", burnAmount))
		}
	case "UpdateToken":
		// Update token last update timestamp
		tokenStandard := txData.Inputs["tokenStandard"]
		if tokenStandard != "" {
			i.repos.Token.UpdateLastUpdateTimestampBatch(batch, tokenStandard, int64(m.TimestampUnix))
			i.logger.Debug("token update recorded",
				zap.String("token", tokenStandard),
				zap.Int64("timestamp", int64(m.TimestampUnix)))
		}
	}
}
