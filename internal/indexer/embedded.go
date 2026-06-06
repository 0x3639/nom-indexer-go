package indexer

import (
	"context"
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/0x3639/znn-sdk-go/embedded"
	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
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
	case models.HtlcAddress:
		i.indexHtlcContract(ctx, batch, block, txData, m)
	case models.SwapAddress:
		i.indexSwapContract(ctx, batch, block, txData, m)
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
					slotCostQsr := safeBigIntToInt64(descendant.Amount, i.logger,
						"pillar slot cost overflow",
						zap.String("name", name),
						zap.String("owner", ownerAddress))
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
		// Update account delegation + append to delegation history (close
		// the previous open interval if any, then open a new one).
		pillarName := txData.Inputs["name"]
		if pillarName != "" && block.PairedAccountBlock != nil {
			pillarOwner := i.getPillarOwnerAddress(pillarName)
			if pillarOwner != "" {
				delegatorAddress := block.PairedAccountBlock.Address.String()
				ts := int64(m.TimestampUnix)
				i.repos.Account.UpdateDelegateBatch(batch, delegatorAddress, pillarOwner, ts)
				i.repos.Delegation.CloseActiveBatch(batch, delegatorAddress, ts)
				i.repos.Delegation.OpenBatch(batch, delegatorAddress, pillarOwner, ts)
				i.logger.Debug("delegation recorded",
					zap.String("delegator", delegatorAddress),
					zap.String("pillar", pillarName))
			}
		}
	case "Undelegate":
		// Clear account delegation + close any open delegation interval.
		if block.PairedAccountBlock != nil {
			delegatorAddress := block.PairedAccountBlock.Address.String()
			i.repos.Account.UpdateDelegateBatch(batch, delegatorAddress, "", 0)
			i.repos.Delegation.CloseActiveBatch(batch, delegatorAddress, int64(m.TimestampUnix))
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
			znnAmount := safeBigIntToInt64(block.PairedAccountBlock.Amount, i.logger,
				"stake amount overflow",
				zap.String("stakeID", stakeID))
			stake := &models.Stake{
				ID:                  stakeID,
				Address:             block.PairedAccountBlock.Address.String(),
				ZnnAmount:           znnAmount,
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
			qsrAmount := safeBigIntToInt64(block.PairedAccountBlock.Amount, i.logger,
				"fusion qsr amount overflow",
				zap.String("fusionID", fusionID))
			fusion := &models.Fusion{
				ID:                fusionID,
				Address:           block.PairedAccountBlock.Address.String(),
				Beneficiary:       beneficiary,
				QsrAmount:         qsrAmount,
				MomentumTimestamp: int64(m.TimestampUnix),
				MomentumHeight:    int64(m.Height),
				MomentumHash:      m.Hash.String(),
				ExpirationHeight:  int64(m.Height) + models.FusionExpirationBlocks,
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
	case "Mint":
		// A Mint contract-receive on the token contract. Inputs carry the
		// destination token, amount, and receiver; the paired send block's
		// address is the issuer (typically an embedded reward contract or a
		// token owner).
		if block.PairedAccountBlock == nil {
			return
		}
		tokenStandard := txData.Inputs["tokenStandard"]
		amountStr := txData.Inputs["amount"]
		receiver := txData.Inputs["receiveAddress"]
		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			i.logger.Warn("invalid mint amount",
				zap.String("amount", amountStr),
				zap.String("hash", block.Hash.String()),
				zap.Error(err))
			return
		}
		mint := &models.TokenMint{
			AccountBlockHash:  block.Hash.String(),
			MomentumHeight:    int64(m.Height),
			MomentumTimestamp: int64(m.TimestampUnix),
			TokenStandard:     tokenStandard,
			Issuer:            block.PairedAccountBlock.Address.String(),
			Receiver:          receiver,
			Amount:            amount,
		}
		i.repos.TokenEvent.InsertMintBatch(batch, mint)
		i.logger.Debug("token mint recorded",
			zap.String("token", tokenStandard),
			zap.String("issuer", mint.Issuer),
			zap.String("receiver", mint.Receiver),
			zap.Int64("amount", amount))

	case "Burn":
		// A Burn contract-receive on the token contract. The paired send
		// carries the actual amount and token; the send's address is the
		// burner.
		if block.PairedAccountBlock == nil {
			return
		}
		tokenStandard := block.PairedAccountBlock.TokenStandard.String()
		burnAmount := safeBigIntToInt64(block.PairedAccountBlock.Amount, i.logger,
			"token burn amount overflow",
			zap.String("hash", block.Hash.String()),
			zap.String("token", tokenStandard))
		burner := block.PairedAccountBlock.Address.String()
		i.repos.TokenEvent.InsertBurnBatch(batch, &models.TokenBurn{
			AccountBlockHash:  block.Hash.String(),
			MomentumHeight:    int64(m.Height),
			MomentumTimestamp: int64(m.TimestampUnix),
			TokenStandard:     tokenStandard,
			Burner:            burner,
			Amount:            burnAmount,
		})
		i.repos.Token.UpdateBurnAmountBatch(batch, tokenStandard, burnAmount)
		i.logger.Debug("token burn recorded",
			zap.String("token", tokenStandard),
			zap.String("burner", burner),
			zap.Int64("amount", burnAmount))

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

// indexSwapContract handles legacy genesis-swap RetrieveAssets claims.
//
// A claimant calls RetrieveAssets(publicKey, signature) on the swap contract;
// the contract disburses the claimant's remaining genesis ZNN and/or QSR as
// descendant blocks. Those descendants are Token.Mint calls to the token
// contract whose own block-level Amount is 0 and whose block-level
// TokenStandard is ZNN for BOTH the ZNN and QSR payout (a go-zenon quirk, see
// vm/embedded/implementation/swap.go). The real token+amount live in the Mint
// call data: Mint(tokenStandard, amount, receiveAddress). So we decode each
// descendant's Data with the Token ABI and read the amount from the Mint
// params, bucketing by the Mint's tokenStandard. We key the swap_retrievals row
// by the paired send-block hash. Authoritative remaining balances come from the
// swap_assets snapshot (syncSwapAssets).
func (i *Indexer) indexSwapContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	if txData.Method != "RetrieveAssets" || block.PairedAccountBlock == nil {
		return
	}
	paired := block.PairedAccountBlock

	var znn, qsr int64
	for _, d := range block.DescendantBlocks {
		dec := i.tryDecodeFromAbi(d.Data, embedded.Token)
		if dec == nil || dec.Method != "Mint" {
			continue
		}
		amt, ok := new(big.Int).SetString(dec.Inputs["amount"], 10)
		if !ok {
			i.logger.Warn("swap retrieval: unparseable mint amount",
				zap.String("swapID", paired.Hash.String()),
				zap.String("amount", dec.Inputs["amount"]))
			continue
		}
		v := safeBigIntToInt64(amt, i.logger,
			"swap retrieval amount overflow",
			zap.String("swapID", paired.Hash.String()))
		switch dec.Inputs["tokenStandard"] {
		case models.ZnnTokenStandard:
			znn += v
		case models.QsrTokenStandard:
			qsr += v
		}
	}

	i.repos.Swap.InsertRetrievalBatch(batch, &models.SwapRetrieval{
		ID:                paired.Hash.String(),
		Address:           paired.Address.String(),
		PublicKey:         txData.Inputs["publicKey"],
		ZnnAmount:         znn,
		QsrAmount:         qsr,
		MomentumHeight:    int64(m.Height),
		MomentumTimestamp: int64(m.TimestampUnix),
	})
	i.logger.Debug("swap retrieval recorded",
		zap.String("swapID", paired.Hash.String()),
		zap.String("address", paired.Address.String()),
		zap.Int64("znn", znn),
		zap.Int64("qsr", qsr))
}

// bytesToHex hex-encodes a byte slice; empty/nil yields "".
func bytesToHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

// indexHtlcContract handles HTLC Create / Unlock / Reclaim events.
//
// HTLC blocks arrive as ContractReceive on the HTLC address paired with a user
// send. The entry id is the Create *send*-block hash (paired.Hash): go-zenon
// sets HtlcInfo.Id = sendBlock.Hash (vm/embedded/implementation/htlc.go), and
// Unlock/Reclaim reference that send-block hash as their target id (verified
// against mainnet: 571/571 settlements match the send-block hash, 0 match the
// receive-block hash). Create carries the lock params + the send's
// amount/token/sender (all from paired); Unlock and Reclaim carry the target id
// (and Unlock a preimage) to settle the entry.
func (i *Indexer) indexHtlcContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	if block.PairedAccountBlock == nil {
		return
	}
	paired := block.PairedAccountBlock

	switch txData.Method {
	case "Create":
		id := paired.Hash.String()

		expirationStr := txData.Inputs["expirationTime"]
		expiration, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			i.logger.Warn("invalid htlc expirationTime",
				zap.String("htlcID", id), zap.String("expirationTime", expirationStr), zap.Error(err))
			expiration = 0
		}

		hashTypeStr := txData.Inputs["hashType"]
		hashType, err := strconv.Atoi(hashTypeStr)
		if err != nil {
			i.logger.Warn("invalid htlc hashType",
				zap.String("htlcID", id), zap.String("hashType", hashTypeStr), zap.Error(err))
			hashType = 0
		}

		keyMaxSizeStr := txData.Inputs["keyMaxSize"]
		keyMaxSize, err := strconv.Atoi(keyMaxSizeStr)
		if err != nil {
			i.logger.Warn("invalid htlc keyMaxSize",
				zap.String("htlcID", id), zap.String("keyMaxSize", keyMaxSizeStr), zap.Error(err))
			keyMaxSize = 0
		}

		amount := safeBigIntToInt64(paired.Amount, i.logger,
			"htlc amount overflow", zap.String("htlcID", id))

		h := &models.Htlc{
			ID:                  id,
			TimeLockedAddress:   paired.Address.String(), // sender can Reclaim
			HashLockedAddress:   txData.Inputs["hashLocked"],
			TokenStandard:       paired.TokenStandard.String(),
			Amount:              amount,
			ExpirationTimestamp: expiration,
			HashType:            int16(hashType),
			KeyMaxSize:          int16(keyMaxSize),
			// hashLock is ABI `bytes`; formatArg hands it back as a raw byte
			// string, so encode unconditionally — never treat hex-looking raw
			// bytes (e.g. the bytes "deadbeef") as already-hex.
			HashLock:                  bytesToHex([]byte(txData.Inputs["hashLock"])),
			Status:                    int16(models.HtlcStatusActive),
			CreationMomentumHeight:    int64(m.Height),
			CreationMomentumTimestamp: int64(m.TimestampUnix),
		}
		i.repos.Htlc.InsertBatch(batch, h)

	case "Unlock":
		id := txData.Inputs["id"]
		if id == "" {
			return
		}
		// preimage is ABI `bytes`; encode unconditionally (see Create/hashLock).
		i.repos.Htlc.SettleBatch(batch, id, int16(models.HtlcStatusUnlocked),
			bytesToHex([]byte(txData.Inputs["preimage"])), int64(m.Height), int64(m.TimestampUnix))

	case "Reclaim":
		id := txData.Inputs["id"]
		if id == "" {
			return
		}
		i.repos.Htlc.SettleBatch(batch, id, int16(models.HtlcStatusReclaimed),
			"", int64(m.Height), int64(m.TimestampUnix))
	}
}
