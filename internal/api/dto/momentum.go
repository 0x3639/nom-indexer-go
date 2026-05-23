package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// Momentum is the JSON shape returned by /api/v1/momentums endpoints.
// Height and Timestamp are safely below 2^53 so they ride as JSON numbers.
type Momentum struct {
	Height        uint64 `json:"height"`
	Hash          string `json:"hash"`
	Timestamp     int64  `json:"timestamp"`
	TxCount       int    `json:"tx_count"`
	Producer      string `json:"producer"`
	ProducerOwner string `json:"producer_owner,omitempty"`
	ProducerName  string `json:"producer_name,omitempty"`
}

// FromMomentum converts a model into the wire shape.
func FromMomentum(m *models.Momentum) *Momentum {
	if m == nil {
		return nil
	}
	return &Momentum{
		Height:        m.Height,
		Hash:          m.Hash,
		Timestamp:     m.Timestamp,
		TxCount:       m.TxCount,
		Producer:      m.Producer,
		ProducerOwner: m.ProducerOwner,
		ProducerName:  m.ProducerName,
	}
}

// FromMomentums converts a slice of models. nil input yields an empty
// slice so JSON renders as [] rather than null.
func FromMomentums(in []*models.Momentum) []*Momentum {
	out := make([]*Momentum, 0, len(in))
	for _, m := range in {
		if d := FromMomentum(m); d != nil {
			out = append(out, d)
		}
	}
	return out
}
