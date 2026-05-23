package dto

import (
	"encoding/json"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// AccountBlock is the JSON shape for a transaction (account-block in
// Zenon's dual-ledger terminology). Amount is stringified via Amount;
// every other int64 here is a height or timestamp that stays well below
// 2^53.
type AccountBlock struct {
	Hash               string          `json:"hash"`
	MomentumHash       string          `json:"momentum_hash"`
	MomentumTimestamp  int64           `json:"momentum_timestamp"`
	MomentumHeight     int64           `json:"momentum_height"`
	BlockType          int16           `json:"block_type"`
	Height             int64           `json:"height"`
	Address            string          `json:"address"`
	ToAddress          string          `json:"to_address,omitempty"`
	Amount             Amount          `json:"amount"`
	TokenStandard      string          `json:"token_standard,omitempty"`
	Data               string          `json:"data,omitempty"`
	Method             string          `json:"method,omitempty"`
	Input              json.RawMessage `json:"input,omitempty"`
	PairedAccountBlock string          `json:"paired_account_block,omitempty"`
	DescendantOf       string          `json:"descendant_of,omitempty"`
}

func FromAccountBlock(ab *models.AccountBlock) *AccountBlock {
	if ab == nil {
		return nil
	}
	return &AccountBlock{
		Hash:               ab.Hash,
		MomentumHash:       ab.MomentumHash,
		MomentumTimestamp:  ab.MomentumTimestamp,
		MomentumHeight:     ab.MomentumHeight,
		BlockType:          ab.BlockType,
		Height:             ab.Height,
		Address:            ab.Address,
		ToAddress:          ab.ToAddress,
		Amount:             AmountFromInt64(ab.Amount),
		TokenStandard:      ab.TokenStandard,
		Data:               ab.Data,
		Method:             ab.Method,
		Input:              ab.Input,
		PairedAccountBlock: ab.PairedAccountBlock,
		DescendantOf:       ab.DescendantOf,
	}
}

func FromAccountBlocks(in []*models.AccountBlock) []*AccountBlock {
	out := make([]*AccountBlock, 0, len(in))
	for _, ab := range in {
		if d := FromAccountBlock(ab); d != nil {
			out = append(out, d)
		}
	}
	return out
}
