package indexer

import (
	"bytes"
	"fmt"

	"github.com/0x3639/znn-sdk-go/abi"
	"github.com/0x3639/znn-sdk-go/embedded"
	rpcapi "github.com/zenon-network/go-zenon/rpc/api"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// tryDecodeTxData attempts to decode transaction data from an account block
func (i *Indexer) tryDecodeTxData(block *rpcapi.AccountBlock) *models.TxData {
	if len(block.Data) == 0 {
		return nil
	}

	toAddress := block.ToAddress.String()

	// Only decode for embedded contracts
	if !models.IsEmbeddedContract(toAddress) {
		return nil
	}

	// Try common definitions first
	txData := i.tryDecodeFromAbi(block.Data, embedded.Common)
	if txData != nil && txData.Method != "" {
		return txData
	}

	// Try contract-specific definitions
	var contractAbi *abi.Abi
	switch toAddress {
	case models.PlasmaAddress:
		contractAbi = embedded.Plasma
	case models.PillarAddress:
		contractAbi = embedded.Pillar
	case models.TokenAddress:
		contractAbi = embedded.Token
	case models.SentinelAddress:
		contractAbi = embedded.Sentinel
	case models.StakeAddress:
		contractAbi = embedded.Stake
	case models.AcceleratorAddress:
		contractAbi = embedded.Accelerator
	case models.SwapAddress:
		contractAbi = embedded.Swap
	case models.LiquidityAddress:
		contractAbi = embedded.Liquidity
	case models.BridgeAddress:
		contractAbi = embedded.Bridge
	case models.HtlcAddress:
		contractAbi = embedded.Htlc
	default:
		return nil
	}

	if contractAbi == nil {
		return nil
	}

	txData = i.tryDecodeFromAbi(block.Data, contractAbi)
	if txData == nil || txData.Method == "" {
		i.logger.Debug("unable to decode transaction data",
			zap.String("hash", block.Hash.String()),
			zap.String("toAddress", toAddress))
		return nil
	}

	i.logger.Debug("decoded transaction",
		zap.String("method", txData.Method),
		zap.String("hash", block.Hash.String()))

	return txData
}

// tryDecodeFromAbi tries to decode data using the SDK's ABI
func (i *Indexer) tryDecodeFromAbi(data []byte, contractAbi *abi.Abi) *models.TxData {
	if contractAbi == nil || len(data) < 4 {
		return nil
	}

	// Extract method signature (first 4 bytes)
	methodSig := data[:4]

	// Find matching function entry
	for _, entry := range contractAbi.Entries {
		if entry.Type != abi.Function {
			continue
		}

		// Check if signature matches
		entrySig := entry.EncodeSignature()
		if len(entrySig) < 4 {
			continue
		}

		if bytes.Equal(methodSig, entrySig[:4]) {
			txData := models.NewTxData()
			txData.Method = entry.Name

			// Decode inputs if any
			if len(entry.Inputs) > 0 && len(data) > 4 {
				// Use the Abi's DecodeFunction which handles decoding
				args, err := contractAbi.DecodeFunction(data)
				if err != nil {
					i.logger.Debug("failed to decode inputs",
						zap.String("method", entry.Name),
						zap.Error(err))
					return txData
				}

				for idx, param := range entry.Inputs {
					if idx < len(args) {
						txData.Inputs[param.Name] = formatArg(args[idx])
					}
				}
			}

			return txData
		}
	}

	return nil
}

// formatArg converts an argument to string
func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
