package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

type Fusion struct {
	ID                string `json:"id"`
	Address           string `json:"address"`
	Beneficiary       string `json:"beneficiary"`
	MomentumHash      string `json:"momentum_hash"`
	MomentumTimestamp int64  `json:"momentum_timestamp"`
	MomentumHeight    int64  `json:"momentum_height"`
	QsrAmount         Amount `json:"qsr_amount"`
	ExpirationHeight  int64  `json:"expiration_height"`
	IsActive          bool   `json:"is_active"`
	CancelID          string `json:"cancel_id,omitempty"`
}

func FromFusion(f *models.Fusion) *Fusion {
	if f == nil {
		return nil
	}
	return &Fusion{
		ID:                f.ID,
		Address:           f.Address,
		Beneficiary:       f.Beneficiary,
		MomentumHash:      f.MomentumHash,
		MomentumTimestamp: f.MomentumTimestamp,
		MomentumHeight:    f.MomentumHeight,
		QsrAmount:         AmountFromInt64(f.QsrAmount),
		ExpirationHeight:  f.ExpirationHeight,
		IsActive:          f.IsActive,
		CancelID:          f.CancelID,
	}
}

func FromFusions(in []*models.Fusion) []*Fusion {
	out := make([]*Fusion, 0, len(in))
	for _, f := range in {
		if d := FromFusion(f); d != nil {
			out = append(out, d)
		}
	}
	return out
}
