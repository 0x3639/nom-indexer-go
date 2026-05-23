package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// Token is the JSON shape for a ZTS token. Amount-valued fields ship as
// strings — see Amount. HolderCount/TransactionCount stay numeric since
// they cannot realistically exceed 2^53.
type Token struct {
	TokenStandard       string `json:"token_standard"`
	Name                string `json:"name"`
	Symbol              string `json:"symbol"`
	Domain              string `json:"domain,omitempty"`
	Decimals            int    `json:"decimals"`
	Owner               string `json:"owner"`
	TotalSupply         Amount `json:"total_supply"`
	MaxSupply           Amount `json:"max_supply"`
	IsBurnable          bool   `json:"is_burnable"`
	IsMintable          bool   `json:"is_mintable"`
	IsUtility           bool   `json:"is_utility"`
	TotalBurned         Amount `json:"total_burned"`
	LastUpdateTimestamp int64  `json:"last_update_timestamp"`
	HolderCount         int64  `json:"holder_count"`
	TransactionCount    int64  `json:"transaction_count"`
}

func FromToken(t *models.Token) *Token {
	if t == nil {
		return nil
	}
	return &Token{
		TokenStandard:       t.TokenStandard,
		Name:                t.Name,
		Symbol:              t.Symbol,
		Domain:              t.Domain,
		Decimals:            t.Decimals,
		Owner:               t.Owner,
		TotalSupply:         AmountFromInt64(t.TotalSupply),
		MaxSupply:           AmountFromInt64(t.MaxSupply),
		IsBurnable:          t.IsBurnable,
		IsMintable:          t.IsMintable,
		IsUtility:           t.IsUtility,
		TotalBurned:         AmountFromInt64(t.TotalBurned),
		LastUpdateTimestamp: t.LastUpdateTimestamp,
		HolderCount:         t.HolderCount,
		TransactionCount:    t.TransactionCount,
	}
}

func FromTokens(in []*models.Token) []*Token {
	out := make([]*Token, 0, len(in))
	for _, t := range in {
		if d := FromToken(t); d != nil {
			out = append(out, d)
		}
	}
	return out
}
