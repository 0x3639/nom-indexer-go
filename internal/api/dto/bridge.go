package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

// WrapTokenRequest is a Zenon→external chain transfer record.
type WrapTokenRequest struct {
	ID                      string `json:"id"`
	NetworkClass            int    `json:"network_class"`
	ChainID                 int    `json:"chain_id"`
	ToAddress               string `json:"to_address"`
	TokenStandard           string `json:"token_standard"`
	TokenAddress            string `json:"token_address"`
	Amount                  Amount `json:"amount"`
	Fee                     Amount `json:"fee"`
	Signature               string `json:"signature,omitempty"`
	CreationMomentumHeight  int64  `json:"creation_momentum_height"`
	ConfirmationsToFinality int    `json:"confirmations_to_finality"`
}

func FromWrapTokenRequest(w *models.WrapTokenRequest) *WrapTokenRequest {
	if w == nil {
		return nil
	}
	return &WrapTokenRequest{
		ID:                      w.ID,
		NetworkClass:            w.NetworkClass,
		ChainID:                 w.ChainID,
		ToAddress:               w.ToAddress,
		TokenStandard:           w.TokenStandard,
		TokenAddress:            w.TokenAddress,
		Amount:                  AmountFromInt64(w.Amount),
		Fee:                     AmountFromInt64(w.Fee),
		Signature:               w.Signature,
		CreationMomentumHeight:  w.CreationMomentumHeight,
		ConfirmationsToFinality: w.ConfirmationsToFinality,
	}
}

func FromWrapTokenRequests(in []*models.WrapTokenRequest) []*WrapTokenRequest {
	out := make([]*WrapTokenRequest, 0, len(in))
	for _, w := range in {
		if d := FromWrapTokenRequest(w); d != nil {
			out = append(out, d)
		}
	}
	return out
}

// UnwrapTokenRequest is an external chain→Zenon transfer record.
type UnwrapTokenRequest struct {
	TransactionHash            string `json:"transaction_hash"`
	LogIndex                   int64  `json:"log_index"`
	NetworkClass               int    `json:"network_class"`
	ChainID                    int    `json:"chain_id"`
	ToAddress                  string `json:"to_address"`
	TokenStandard              string `json:"token_standard"`
	TokenAddress               string `json:"token_address"`
	Amount                     Amount `json:"amount"`
	Signature                  string `json:"signature,omitempty"`
	RegistrationMomentumHeight int64  `json:"registration_momentum_height"`
	Redeemed                   bool   `json:"redeemed"`
	Revoked                    bool   `json:"revoked"`
	RedeemableIn               int64  `json:"redeemable_in"`
}

func FromUnwrapTokenRequest(u *models.UnwrapTokenRequest) *UnwrapTokenRequest {
	if u == nil {
		return nil
	}
	return &UnwrapTokenRequest{
		TransactionHash:            u.TransactionHash,
		LogIndex:                   u.LogIndex,
		NetworkClass:               u.NetworkClass,
		ChainID:                    u.ChainID,
		ToAddress:                  u.ToAddress,
		TokenStandard:              u.TokenStandard,
		TokenAddress:               u.TokenAddress,
		Amount:                     AmountFromInt64(u.Amount),
		Signature:                  u.Signature,
		RegistrationMomentumHeight: u.RegistrationMomentumHeight,
		Redeemed:                   u.Redeemed,
		Revoked:                    u.Revoked,
		RedeemableIn:               u.RedeemableIn,
	}
}

func FromUnwrapTokenRequests(in []*models.UnwrapTokenRequest) []*UnwrapTokenRequest {
	out := make([]*UnwrapTokenRequest, 0, len(in))
	for _, u := range in {
		if d := FromUnwrapTokenRequest(u); d != nil {
			out = append(out, d)
		}
	}
	return out
}
