package indexer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/0x3639/znn-sdk-go/utils"
	"github.com/jackc/pgx/v5"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// genesisBalanceUpdateThreshold skips per-address balance fetching for
// momentums with this many or more transactions. Genesis has tens of thousands
// of txs and per-address GetAccountInfoByAddress calls would be prohibitive.
const genesisBalanceUpdateThreshold = 1000

// safeBigIntToInt64 converts a *big.Int to int64, capping at math.MaxInt64 if
// the value overflows. Returns 0 if v is nil. Logs a warning on cap.
//
// Storage columns for amounts/balances/supplies are BIGINT, so out-of-range
// values are silently truncated. Reconsider NUMERIC(78,0) if any token's
// supply approaches 9.22e18 satoshi.
func safeBigIntToInt64(v *big.Int, logger *zap.Logger, msg string, fields ...zap.Field) int64 {
	if v == nil {
		return 0
	}
	if v.IsInt64() {
		return v.Int64()
	}
	logger.Warn(msg, fields...)
	return math.MaxInt64
}

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

		// Skip per-address balance fetching for momentums with too many txs:
		// genesis has tens of thousands and per-address GetAccountInfoByAddress
		// would be prohibitively slow.
		if m.Height > 1 && len(m.Content) < genesisBalanceUpdateThreshold {
			if err := i.updateBalances(ctx, batch, m.Content, int64(m.TimestampUnix)); err != nil {
				i.logger.Warn("failed to update balances", zap.Error(err))
			}
		}
	}

	// Get pillar info for this momentum
	producerOwner, producerName := i.getPillarInfoForProducer(ctx, m.Producer.String(), m.Height)

	// Increment pillar momentum count
	if producerOwner != "" {
		i.repos.Pillar.IncrementMomentumCountBatch(batch, producerOwner)
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

	// Queue NOTIFY in the same transaction as the row writes. Postgres
	// delivers NOTIFY only when the transaction commits, so live stream
	// clients cannot see an event for rolled-back data, and a pg_notify
	// failure rolls the whole momentum back for the normal retry path.
	if err := queueMomentumNotify(batch, momentum); err != nil {
		return fmt.Errorf("queue momentum %d notify: %w", m.Height, err)
	}

	// Run the batch inside a transaction so partial failures roll back and the
	// caller can retry the height instead of advancing past corrupted state.
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for momentum %d: %w", m.Height, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	results := tx.SendBatch(ctx, batch)
	var batchErr error
	for j := 0; j < batch.Len(); j++ {
		if _, err := results.Exec(); err != nil {
			if batchErr == nil {
				batchErr = fmt.Errorf("batch op %d: %w", j, err)
			}
			i.logger.Warn("batch operation failed", zap.Int("index", j), zap.Error(err))
		}
	}
	if closeErr := results.Close(); closeErr != nil && batchErr == nil {
		batchErr = fmt.Errorf("close batch results: %w", closeErr)
	}
	if batchErr != nil {
		return fmt.Errorf("momentum %d batch failed: %w", m.Height, batchErr)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit momentum %d: %w", m.Height, err)
	}
	committed = true

	i.logger.Debug("processed momentum",
		zap.Uint64("height", m.Height),
		zap.Duration("duration", time.Since(start)))

	return nil
}

// queueMomentumNotify appends a NOTIFY momentum_new statement with a snake_case JSON
// payload — same field names as the API's dto.Momentum wire shape, so
// the stream hub can unmarshal directly into the type it ships to
// WebSocket clients. (models.Momentum lacks json tags by design — the
// indexer-side schema is decoupled from the API wire format.)
//
// This must be queued inside processMomentum's transaction. Postgres
// emits NOTIFY messages only after commit, so subscribers are woken
// exactly when the committed row is visible; rollback suppresses the
// notification automatically.
func queueMomentumNotify(batch *pgx.Batch, m *models.Momentum) error {
	payload, err := json.Marshal(map[string]interface{}{
		"height":         m.Height,
		"hash":           m.Hash,
		"timestamp":      m.Timestamp,
		"tx_count":       m.TxCount,
		"producer":       m.Producer,
		"producer_owner": m.ProducerOwner,
		"producer_name":  m.ProducerName,
	})
	if err != nil {
		return fmt.Errorf("marshal notify payload: %w", err)
	}
	// pg_notify is the function form; takes payload as a parameter so
	// we don't have to escape JSON manually.
	batch.Queue(`SELECT pg_notify('momentum_new', $1::text)`, string(payload))
	return nil
}

// updateBalances updates balances for all addresses in a momentum
func (i *Indexer) updateBalances(ctx context.Context, batch *pgx.Batch, headers []*types.AccountHeader, momentumTimestamp int64) error {
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
					// Check for Int64 overflow before conversion. Balance columns are BIGINT,
					// so values >math.MaxInt64 are silently capped. ZNN/QSR amounts use 1e8
					// satoshi scaling and are well below int64 max today; reconsider if any
					// token's supply approaches 9.22e18 satoshi.
					balanceInt64 := safeBigIntToInt64(balanceInfo.Balance, i.logger,
						"balance overflow",
						zap.String("address", header.Address.String()),
						zap.String("token", tokenStandard.String()))
					balance := &models.Balance{
						Address:              header.Address.String(),
						TokenStandard:        tokenStandard.String(),
						Balance:              balanceInt64,
						LastUpdatedTimestamp: momentumTimestamp,
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

		amountInt64 := safeBigIntToInt64(block.Amount, i.logger,
			"amount overflow",
			zap.String("hash", block.Hash.String()))

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

		// Track ZNN/QSR flow + activity on the involved accounts. Sends bump
		// znn_sent/qsr_sent on the sender; receives bump znn_received/qsr_received
		// on the receiver. Both always update first/last activity.
		ts := int64(m.TimestampUnix)
		sender := block.Address.String()
		tokenStd := block.TokenStandard.String()
		switch block.BlockType {
		case utils.BlockTypeUserSend, utils.BlockTypeContractSend:
			i.repos.Account.AddSendBatch(batch, sender, tokenStd, amountInt64, ts)
		case utils.BlockTypeUserReceive, utils.BlockTypeContractReceive, utils.BlockTypeGenesisReceive:
			i.repos.Account.AddReceiveBatch(batch, sender, tokenStd, amountInt64, ts)
			// At genesis (height 1), seed genesis_*_balance from the received amount.
			if block.BlockType == utils.BlockTypeGenesisReceive && m.Height == 1 {
				i.repos.Account.SetGenesisBalanceBatch(batch, sender, tokenStd, amountInt64)
			}
		}

		// Update paired block reference
		if block.PairedAccountBlock != nil {
			i.repos.AccountBlock.UpdatePairedBlockBatch(batch, block.PairedAccountBlock.Hash.String(), block.Hash.String())
		}

		// Update descendant blocks
		for _, descendant := range block.DescendantBlocks {
			i.repos.AccountBlock.UpdateDescendantOfBatch(batch, descendant.Hash.String(), block.Hash.String())
		}

		// Process embedded contract events: a contract-receive block paired
		// with a user-send call into one of our tracked embedded contracts.
		if block.BlockType == utils.BlockTypeContractReceive &&
			block.PairedAccountBlock != nil &&
			models.IsEmbeddedContract(block.Address.String()) {

			pairedTxData := i.tryDecodeTxData(block.PairedAccountBlock)
			if pairedTxData != nil {
				i.indexEmbeddedContracts(ctx, batch, block, pairedTxData, m)
			}
		}

		// Process reward receive transactions: a user-receive block paired
		// with a contract-send from a reward source. (Note: prior to this fix,
		// the literals here were wrong — BlockType 4 was used for "UserReceive"
		// and 6 for "ContractSend", neither of which match the SDK constants.
		// The branch never fired, so reward_transactions / cumulative_rewards
		// stayed empty for non-liquidity rewards.)
		if block.PairedAccountBlock != nil && block.BlockType == utils.BlockTypeUserReceive {
			if block.PairedAccountBlock.Address.String() == models.LiquidityTreasuryAddress {
				i.indexLiquidityReward(batch, block, m)
			} else if block.PairedAccountBlock.BlockType == utils.BlockTypeContractSend &&
				block.ToAddress.String() == models.EmptyAddress &&
				block.TokenStandard.String() == models.EmptyTokenStandard {
				i.indexReceivedReward(ctx, batch, block, m)
			}
		}

		// Update token info from TokenInfo field
		if block.TokenInfo != nil {
			tokenStdStr := block.TokenInfo.ZenonTokenStandard.String()
			totalSupply := safeBigIntToInt64(block.TokenInfo.TotalSupply, i.logger,
				"total_supply overflow",
				zap.String("token", tokenStdStr))
			maxSupply := safeBigIntToInt64(block.TokenInfo.MaxSupply, i.logger,
				"max_supply overflow",
				zap.String("token", tokenStdStr))

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
