package indexer

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// processMomentum processes a single momentum and all its account blocks
func (i *Indexer) processMomentum(ctx context.Context, m *api.Momentum) error {
	start := time.Now()

	batch := &pgx.Batch{}

	// Process account blocks if any
	if len(m.Content) > 0 {
		// Process each account block
		if err := i.processAccountBlocks(ctx, batch, m); err != nil {
			return fmt.Errorf("failed to process account blocks: %w", err)
		}

		// Update balances (skip for genesis due to large number of transactions)
		// Balance updates are done per-block which can be slow for genesis
		if m.Height > 1 && len(m.Content) < 1000 {
			if err := i.updateBalances(ctx, batch, m.Content); err != nil {
				i.logger.Warn("failed to update balances", zap.Error(err))
			}
		}
	}

	// Get pillar info for this momentum
	producerOwner, producerName := i.getPillarInfoForProducer(ctx, m.Producer.String(), m.Height)

	// Increment pillar momentum count
	if producerOwner != "" {
		if err := i.repos.Pillar.IncrementMomentumCount(ctx, producerOwner); err != nil {
			i.logger.Warn("failed to increment pillar momentum count",
				zap.String("owner", producerOwner),
				zap.Error(err))
		}
	}

	// Insert momentum
	momentum := &models.Momentum{
		Height:        m.Height,
		Hash:          m.Hash.String(),
		Timestamp:     int64(m.TimestampUnix),
		TxCount:       len(m.Content),
		Producer:      m.Producer.String(),
		ProducerOwner: producerOwner,
		ProducerName:  producerName,
	}
	i.repos.Momentum.InsertBatch(ctx, batch, momentum)

	// Execute batch
	results := i.pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	// Check for errors in batch
	for j := 0; j < batch.Len(); j++ {
		if _, err := results.Exec(); err != nil {
			i.logger.Warn("batch operation failed", zap.Int("index", j), zap.Error(err))
		}
	}

	i.logger.Debug("processed momentum",
		zap.Uint64("height", m.Height),
		zap.Duration("duration", time.Since(start)))

	return nil
}

// updateBalances updates balances for all addresses in a momentum
func (i *Indexer) updateBalances(ctx context.Context, batch *pgx.Batch, headers []*types.AccountHeader) error {
	for _, header := range headers {
		accountInfo, err := i.client.LedgerApi.GetAccountInfoByAddress(header.Address)
		if err != nil {
			i.logger.Warn("failed to get account info",
				zap.String("address", header.Address.String()),
				zap.Error(err))
			continue
		}

		if accountInfo.BalanceInfoMap != nil {
			for tokenStandard, balanceInfo := range accountInfo.BalanceInfoMap {
				if balanceInfo.Balance != nil && balanceInfo.Balance.Sign() >= 0 {
					// Check for Int64 overflow before conversion
					var balanceInt64 int64
					if balanceInfo.Balance.IsInt64() {
						balanceInt64 = balanceInfo.Balance.Int64()
					} else {
						i.logger.Warn("balance exceeds int64 max, capping value",
							zap.String("address", header.Address.String()),
							zap.String("token", tokenStandard.String()))
						balanceInt64 = 9223372036854775807 // Max int64
					}
					balance := &models.Balance{
						Address:       header.Address.String(),
						TokenStandard: tokenStandard.String(),
						Balance:       balanceInt64,
					}
					i.repos.Balance.UpsertBatch(batch, balance)
				}
			}
		}
	}
	return nil
}

// processAccountBlocks processes all account blocks in a momentum
func (i *Indexer) processAccountBlocks(ctx context.Context, batch *pgx.Batch, m *api.Momentum) error {
	for _, header := range m.Content {
		block, err := i.client.LedgerApi.GetAccountBlockByHash(header.Hash)
		if err != nil {
			i.logger.Warn("failed to get account block",
				zap.String("hash", header.Hash.String()),
				zap.Error(err))
			continue
		}

		if block == nil {
			continue
		}

		// Decode transaction data if any
		txData := i.tryDecodeTxData(block)

		// Add pillar owner address to inputs for pillar-related transactions
		if block.ToAddress.String() == models.PillarAddress && txData != nil {
			pillarName := txData.Inputs["name"]
			if pillarName != "" {
				method := txData.Method
				if method == "Delegate" || method == "Register" || method == "RegisterLegacy" ||
					method == "Revoke" || method == "UpdatePillar" {
					txData.Inputs["pillarOwner"] = i.getPillarOwnerAddress(pillarName)
				}
			}
		}

		// Insert account
		account := &models.Account{
			Address:    block.Address.String(),
			BlockCount: int64(block.Height),
			PublicKey:  hex.EncodeToString(block.PublicKey),
		}
		i.repos.Account.UpsertBatch(batch, account)

		// Build account block model
		pairedAccountBlock := ""
		if block.PairedAccountBlock != nil {
			pairedAccountBlock = block.PairedAccountBlock.Hash.String()
		}

		data := ""
		if len(block.Data) > 0 {
			data = hex.EncodeToString(block.Data)
		}

		// Safe Int64 conversion for Amount
		amountInt64 := int64(0)
		if block.Amount != nil && block.Amount.IsInt64() {
			amountInt64 = block.Amount.Int64()
		} else if block.Amount != nil {
			i.logger.Warn("amount exceeds int64 max, capping value",
				zap.String("hash", block.Hash.String()))
			amountInt64 = 9223372036854775807
		}

		accountBlock := &models.AccountBlock{
			Hash:               block.Hash.String(),
			MomentumHash:       m.Hash.String(),
			MomentumTimestamp:  int64(m.TimestampUnix),
			MomentumHeight:     int64(m.Height),
			BlockType:          int16(block.BlockType),
			Height:             int64(block.Height),
			Address:            block.Address.String(),
			ToAddress:          block.ToAddress.String(),
			Amount:             amountInt64,
			TokenStandard:      block.TokenStandard.String(),
			Data:               data,
			PairedAccountBlock: pairedAccountBlock,
		}

		i.repos.AccountBlock.InsertBatch(batch, accountBlock, txData)

		// Update paired block reference
		if block.PairedAccountBlock != nil {
			i.repos.AccountBlock.UpdatePairedBlockBatch(batch, block.PairedAccountBlock.Hash.String(), block.Hash.String())
		}

		// Update descendant blocks
		for _, descendant := range block.DescendantBlocks {
			i.repos.AccountBlock.UpdateDescendantOfBatch(batch, descendant.Hash.String(), block.Hash.String())
		}

		// Process embedded contract events
		if block.BlockType == 5 && // ContractReceive
			block.PairedAccountBlock != nil &&
			models.IsEmbeddedContract(block.Address.String()) {

			pairedTxData := i.tryDecodeTxData(block.PairedAccountBlock)
			if pairedTxData != nil {
				i.indexEmbeddedContracts(ctx, batch, block, pairedTxData, m)
			}
		}

		// Process reward receive transactions
		if block.PairedAccountBlock != nil && block.BlockType == 4 { // UserReceive
			if block.PairedAccountBlock.Address.String() == models.LiquidityTreasuryAddress {
				i.indexLiquidityReward(batch, block, m)
			} else if block.PairedAccountBlock.BlockType == 6 && // ContractSend
				block.ToAddress.String() == models.EmptyAddress &&
				block.TokenStandard.String() == models.EmptyTokenStandard {
				i.indexReceivedReward(ctx, batch, block, m)
			}
		}

		// Update token info from TokenInfo field
		if block.TokenInfo != nil {
			// Safe Int64 conversions for supply values
			totalSupply := int64(0)
			if block.TokenInfo.TotalSupply != nil && block.TokenInfo.TotalSupply.IsInt64() {
				totalSupply = block.TokenInfo.TotalSupply.Int64()
			} else if block.TokenInfo.TotalSupply != nil {
				totalSupply = 9223372036854775807
			}
			maxSupply := int64(0)
			if block.TokenInfo.MaxSupply != nil && block.TokenInfo.MaxSupply.IsInt64() {
				maxSupply = block.TokenInfo.MaxSupply.Int64()
			} else if block.TokenInfo.MaxSupply != nil {
				maxSupply = 9223372036854775807
			}

			token := &models.Token{
				TokenStandard: block.TokenInfo.ZenonTokenStandard.String(),
				Name:          block.TokenInfo.TokenName,
				Symbol:        block.TokenInfo.TokenSymbol,
				Domain:        block.TokenInfo.TokenDomain,
				Decimals:      int(block.TokenInfo.Decimals),
				Owner:         block.TokenInfo.Owner.String(),
				TotalSupply:   totalSupply,
				MaxSupply:     maxSupply,
				IsBurnable:    block.TokenInfo.IsBurnable,
				IsMintable:    block.TokenInfo.IsMintable,
				IsUtility:     block.TokenInfo.IsUtility,
			}
			i.repos.Token.UpsertBatch(batch, token)
			i.repos.Token.IncrementTransactionCountBatch(batch, block.TokenInfo.ZenonTokenStandard.String())
		}
	}

	return nil
}

// getPillarInfoForProducer retrieves pillar info for a producer address at a given height
func (i *Indexer) getPillarInfoForProducer(ctx context.Context, producerAddress string, height uint64) (string, string) {
	// Try to get from pillar updates first
	ownerAddress, name, err := i.repos.PillarUpdate.GetInfoAtHeightByProducer(ctx, producerAddress, int64(height))
	if err == nil && ownerAddress != "" {
		return ownerAddress, name
	}

	// Fall back to current pillar info
	pillar, err := i.repos.Pillar.GetByProducer(ctx, producerAddress)
	if err == nil && pillar != nil {
		return pillar.OwnerAddress, pillar.Name
	}

	return "", ""
}
