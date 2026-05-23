package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// Balance is the JSON shape for a single (address, token, amount) row.
type Balance struct {
	Address              string `json:"address"`
	TokenStandard        string `json:"token_standard"`
	Balance              Amount `json:"balance"`
	LastUpdatedTimestamp int64  `json:"last_updated_timestamp"`
}

func FromBalance(b *models.Balance) *Balance {
	if b == nil {
		return nil
	}
	return &Balance{
		Address:              b.Address,
		TokenStandard:        b.TokenStandard,
		Balance:              AmountFromInt64(b.Balance),
		LastUpdatedTimestamp: b.LastUpdatedTimestamp,
	}
}

// FromBalances yields an empty (not nil) slice so JSON renders [] rather
// than null when there are no balances.
func FromBalances(in []*models.Balance) []*Balance {
	out := make([]*Balance, 0, len(in))
	for _, b := range in {
		if d := FromBalance(b); d != nil {
			out = append(out, d)
		}
	}
	return out
}
