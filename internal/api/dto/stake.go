package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

type Stake struct {
	ID                  string `json:"id"`
	Address             string `json:"address"`
	StartTimestamp      int64  `json:"start_timestamp"`
	ExpirationTimestamp int64  `json:"expiration_timestamp"`
	ZnnAmount           Amount `json:"znn_amount"`
	DurationInSec       int    `json:"duration_in_sec"`
	IsActive            bool   `json:"is_active"`
	CancelID            string `json:"cancel_id,omitempty"`
}

func FromStake(s *models.Stake) *Stake {
	if s == nil {
		return nil
	}
	return &Stake{
		ID:                  s.ID,
		Address:             s.Address,
		StartTimestamp:      s.StartTimestamp,
		ExpirationTimestamp: s.ExpirationTimestamp,
		ZnnAmount:           AmountFromInt64(s.ZnnAmount),
		DurationInSec:       s.DurationInSec,
		IsActive:            s.IsActive,
		CancelID:            s.CancelID,
	}
}

func FromStakes(in []*models.Stake) []*Stake {
	out := make([]*Stake, 0, len(in))
	for _, s := range in {
		if d := FromStake(s); d != nil {
			out = append(out, d)
		}
	}
	return out
}
