package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// CumulativeReward is the rolled-up lifetime reward total for one
// (reward_type, token_standard) bucket per address.
type CumulativeReward struct {
	Address       string `json:"address"`
	RewardType    string `json:"reward_type"`
	Amount        Amount `json:"amount"`
	TokenStandard string `json:"token_standard"`
}

func FromCumulativeReward(c *models.CumulativeReward) *CumulativeReward {
	if c == nil {
		return nil
	}
	return &CumulativeReward{
		Address:       c.Address,
		RewardType:    c.RewardType.String(),
		Amount:        AmountFromInt64(c.Amount),
		TokenStandard: c.TokenStandard,
	}
}

func FromCumulativeRewards(in []*models.CumulativeReward) []*CumulativeReward {
	out := make([]*CumulativeReward, 0, len(in))
	for _, c := range in {
		if d := FromCumulativeReward(c); d != nil {
			out = append(out, d)
		}
	}
	return out
}

// RewardTransaction is a single Collect-side reward receipt.
type RewardTransaction struct {
	Hash              string `json:"hash"`
	Address           string `json:"address"`
	RewardType        string `json:"reward_type"`
	MomentumTimestamp int64  `json:"momentum_timestamp"`
	MomentumHeight    int64  `json:"momentum_height"`
	AccountHeight     int64  `json:"account_height"`
	Amount            Amount `json:"amount"`
	TokenStandard     string `json:"token_standard"`
	SourceAddress     string `json:"source_address,omitempty"`
}

func FromRewardTransaction(rt *models.RewardTransaction) *RewardTransaction {
	if rt == nil {
		return nil
	}
	return &RewardTransaction{
		Hash:              rt.Hash,
		Address:           rt.Address,
		RewardType:        rt.RewardType.String(),
		MomentumTimestamp: rt.MomentumTimestamp,
		MomentumHeight:    rt.MomentumHeight,
		AccountHeight:     rt.AccountHeight,
		Amount:            AmountFromInt64(rt.Amount),
		TokenStandard:     rt.TokenStandard,
		SourceAddress:     rt.SourceAddress,
	}
}

func FromRewardTransactions(in []*models.RewardTransaction) []*RewardTransaction {
	out := make([]*RewardTransaction, 0, len(in))
	for _, rt := range in {
		if d := FromRewardTransaction(rt); d != nil {
			out = append(out, d)
		}
	}
	return out
}
