package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// Account is the JSON shape returned by /api/v1/accounts/{address}.
// All monetary amounts ship as strings — see Amount.
type Account struct {
	Address                  string `json:"address"`
	BlockCount               int64  `json:"block_count"`
	PublicKey                string `json:"public_key,omitempty"`
	Delegate                 string `json:"delegate,omitempty"`
	DelegationStartTimestamp int64  `json:"delegation_start_timestamp,omitempty"`
	GenesisZnnBalance        Amount `json:"genesis_znn_balance"`
	GenesisQsrBalance        Amount `json:"genesis_qsr_balance"`
	ZnnSent                  Amount `json:"znn_sent"`
	ZnnReceived              Amount `json:"znn_received"`
	QsrSent                  Amount `json:"qsr_sent"`
	QsrReceived              Amount `json:"qsr_received"`
	FirstActiveAt            *int64 `json:"first_active_at,omitempty"`
	LastActiveAt             *int64 `json:"last_active_at,omitempty"`
}

// FromAccount maps the model into the wire shape. Returns nil for nil
// input so handlers can pass through repository returns directly.
func FromAccount(a *models.Account) *Account {
	if a == nil {
		return nil
	}
	return &Account{
		Address:                  a.Address,
		BlockCount:               a.BlockCount,
		PublicKey:                a.PublicKey,
		Delegate:                 a.Delegate,
		DelegationStartTimestamp: a.DelegationStartTimestamp,
		GenesisZnnBalance:        AmountFromInt64(a.GenesisZnnBalance),
		GenesisQsrBalance:        AmountFromInt64(a.GenesisQsrBalance),
		ZnnSent:                  AmountFromInt64(a.ZnnSent),
		ZnnReceived:              AmountFromInt64(a.ZnnReceived),
		QsrSent:                  AmountFromInt64(a.QsrSent),
		QsrReceived:              AmountFromInt64(a.QsrReceived),
		FirstActiveAt:            a.FirstActiveAt,
		LastActiveAt:             a.LastActiveAt,
	}
}
