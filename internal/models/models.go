package models

import (
	"encoding/json"
)

// RewardType represents the type of reward
type RewardType int

const (
	RewardTypeUnknown RewardType = iota
	RewardTypeStake
	RewardTypeDelegation
	RewardTypeLiquidity
	RewardTypeSentinel
	RewardTypePillar
)

func (r RewardType) String() string {
	switch r {
	case RewardTypeStake:
		return "Stake"
	case RewardTypeDelegation:
		return "Delegation"
	case RewardTypeLiquidity:
		return "Liquidity"
	case RewardTypeSentinel:
		return "Sentinel"
	case RewardTypePillar:
		return "Pillar"
	default:
		return "Unknown"
	}
}

// Momentum represents a blockchain block
type Momentum struct {
	Height        uint64 `db:"height"`
	Hash          string `db:"hash"`
	Timestamp     int64  `db:"timestamp"`
	TxCount       int    `db:"tx_count"`
	Producer      string `db:"producer"`
	ProducerOwner string `db:"producer_owner"`
	ProducerName  string `db:"producer_name"`
}

// Account represents a user account
type Account struct {
	Address                  string `db:"address"`
	BlockCount               int64  `db:"block_count"`
	PublicKey                string `db:"public_key"`
	Delegate                 string `db:"delegate"`
	DelegationStartTimestamp int64  `db:"delegation_start_timestamp"`
}

// Balance represents a token balance for an account
type Balance struct {
	Address       string `db:"address"`
	TokenStandard string `db:"token_standard"`
	Balance       int64  `db:"balance"`
}

// AccountBlock represents a transaction
type AccountBlock struct {
	Hash               string          `db:"hash"`
	MomentumHash       string          `db:"momentum_hash"`
	MomentumTimestamp  int64           `db:"momentum_timestamp"`
	MomentumHeight     int64           `db:"momentum_height"`
	BlockType          int16           `db:"block_type"`
	Height             int64           `db:"height"`
	Address            string          `db:"address"`
	ToAddress          string          `db:"to_address"`
	Amount             int64           `db:"amount"`
	TokenStandard      string          `db:"token_standard"`
	Data               string          `db:"data"`
	Method             string          `db:"method"`
	Input              json.RawMessage `db:"input"`
	PairedAccountBlock string          `db:"paired_account_block"`
	DescendantOf       string          `db:"descendant_of"`
}

// Token represents a ZTS token
type Token struct {
	TokenStandard       string `db:"token_standard"`
	Name                string `db:"name"`
	Symbol              string `db:"symbol"`
	Domain              string `db:"domain"`
	Decimals            int    `db:"decimals"`
	Owner               string `db:"owner"`
	TotalSupply         int64  `db:"total_supply"`
	MaxSupply           int64  `db:"max_supply"`
	IsBurnable          bool   `db:"is_burnable"`
	IsMintable          bool   `db:"is_mintable"`
	IsUtility           bool   `db:"is_utility"`
	TotalBurned         int64  `db:"total_burned"`
	LastUpdateTimestamp int64  `db:"last_update_timestamp"`
	HolderCount         int64  `db:"holder_count"`
	TransactionCount    int64  `db:"transaction_count"`
}

// Pillar represents a validator
type Pillar struct {
	OwnerAddress                 string  `db:"owner_address"`
	ProducerAddress              string  `db:"producer_address"`
	WithdrawAddress              string  `db:"withdraw_address"`
	Name                         string  `db:"name"`
	Rank                         int     `db:"rank"`
	GiveMomentumRewardPercentage int16   `db:"give_momentum_reward_percentage"`
	GiveDelegateRewardPercentage int16   `db:"give_delegate_reward_percentage"`
	IsRevocable                  bool    `db:"is_revocable"`
	RevokeCooldown               int     `db:"revoke_cooldown"`
	RevokeTimestamp              int64   `db:"revoke_timestamp"`
	Weight                       int64   `db:"weight"`
	EpochProducedMomentums       int16   `db:"epoch_produced_momentums"`
	EpochExpectedMomentums       int16   `db:"epoch_expected_momentums"`
	SlotCostQsr                  int64   `db:"slot_cost_qsr"`
	SpawnTimestamp               int64   `db:"spawn_timestamp"`
	VotingActivity               float32 `db:"voting_activity"`
	ProducedMomentumCount        int64   `db:"produced_momentum_count"`
	IsRevoked                    bool    `db:"is_revoked"`
}

// PillarUpdate represents a historical pillar configuration change
type PillarUpdate struct {
	ID                           int    `db:"id"`
	Name                         string `db:"name"`
	OwnerAddress                 string `db:"owner_address"`
	ProducerAddress              string `db:"producer_address"`
	WithdrawAddress              string `db:"withdraw_address"`
	MomentumTimestamp            int64  `db:"momentum_timestamp"`
	MomentumHeight               int64  `db:"momentum_height"`
	MomentumHash                 string `db:"momentum_hash"`
	GiveMomentumRewardPercentage int16  `db:"give_momentum_reward_percentage"`
	GiveDelegateRewardPercentage int16  `db:"give_delegate_reward_percentage"`
}

// Sentinel represents a network sentinel
type Sentinel struct {
	Owner                 string `db:"owner"`
	RegistrationTimestamp int64  `db:"registration_timestamp"`
	IsRevocable           bool   `db:"is_revocable"`
	RevokeCooldown        string `db:"revoke_cooldown"`
	Active                bool   `db:"active"`
}

// Stake represents a staking entry
type Stake struct {
	ID                  string `db:"id"`
	Address             string `db:"address"`
	StartTimestamp      int64  `db:"start_timestamp"`
	ExpirationTimestamp int64  `db:"expiration_timestamp"`
	ZnnAmount           int64  `db:"znn_amount"`
	DurationInSec       int    `db:"duration_in_sec"`
	IsActive            bool   `db:"is_active"`
	CancelID            string `db:"cancel_id"`
}

// Project represents an Accelerator-Z project
type Project struct {
	ID                  string `db:"id"`
	VotingID            string `db:"voting_id"`
	Owner               string `db:"owner"`
	Name                string `db:"name"`
	Description         string `db:"description"`
	URL                 string `db:"url"`
	ZnnFundsNeeded      int64  `db:"znn_funds_needed"`
	QsrFundsNeeded      int64  `db:"qsr_funds_needed"`
	CreationTimestamp   int64  `db:"creation_timestamp"`
	LastUpdateTimestamp int64  `db:"last_update_timestamp"`
	Status              int16  `db:"status"`
	YesVotes            int16  `db:"yes_votes"`
	NoVotes             int16  `db:"no_votes"`
	TotalVotes          int16  `db:"total_votes"`
}

// ProjectPhase represents a phase of an Accelerator-Z project
type ProjectPhase struct {
	ID                string `db:"id"`
	ProjectID         string `db:"project_id"`
	VotingID          string `db:"voting_id"`
	Name              string `db:"name"`
	Description       string `db:"description"`
	URL               string `db:"url"`
	ZnnFundsNeeded    int64  `db:"znn_funds_needed"`
	QsrFundsNeeded    int64  `db:"qsr_funds_needed"`
	CreationTimestamp int64  `db:"creation_timestamp"`
	AcceptedTimestamp int64  `db:"accepted_timestamp"`
	Status            int16  `db:"status"`
	YesVotes          int16  `db:"yes_votes"`
	NoVotes           int16  `db:"no_votes"`
	TotalVotes        int16  `db:"total_votes"`
}

// Vote represents a pillar vote on a project or phase
type Vote struct {
	ID                int    `db:"id"`
	MomentumHash      string `db:"momentum_hash"`
	MomentumTimestamp int64  `db:"momentum_timestamp"`
	MomentumHeight    int64  `db:"momentum_height"`
	VoterAddress      string `db:"voter_address"`
	ProjectID         string `db:"project_id"`
	PhaseID           string `db:"phase_id"`
	VotingID          string `db:"voting_id"`
	Vote              int16  `db:"vote"`
}

// Fusion represents a plasma fusion entry
type Fusion struct {
	ID                string `db:"id"`
	Address           string `db:"address"`
	Beneficiary       string `db:"beneficiary"`
	MomentumHash      string `db:"momentum_hash"`
	MomentumTimestamp int64  `db:"momentum_timestamp"`
	MomentumHeight    int64  `db:"momentum_height"`
	QsrAmount         int64  `db:"qsr_amount"`
	ExpirationHeight  int64  `db:"expiration_height"`
	IsActive          bool   `db:"is_active"`
	CancelID          string `db:"cancel_id"`
}

// CumulativeReward represents cumulative rewards for an address
type CumulativeReward struct {
	ID            int        `db:"id"`
	Address       string     `db:"address"`
	RewardType    RewardType `db:"reward_type"`
	Amount        int64      `db:"amount"`
	TokenStandard string     `db:"token_standard"`
}

// RewardTransaction represents an individual reward transaction
type RewardTransaction struct {
	Hash              string     `db:"hash"`
	Address           string     `db:"address"`
	RewardType        RewardType `db:"reward_type"`
	MomentumTimestamp int64      `db:"momentum_timestamp"`
	MomentumHeight    int64      `db:"momentum_height"`
	AccountHeight     int64      `db:"account_height"`
	Amount            int64      `db:"amount"`
	TokenStandard     string     `db:"token_standard"`
	SourceAddress     string     `db:"source_address"`
}

// TxData represents decoded transaction data
type TxData struct {
	Method string            `json:"method"`
	Inputs map[string]string `json:"inputs"`
}

// NewTxData creates a new TxData instance
func NewTxData() *TxData {
	return &TxData{
		Inputs: make(map[string]string),
	}
}

// WrapTokenRequest represents a request to wrap tokens from Zenon to an external chain
type WrapTokenRequest struct {
	ID                      string `db:"id"`
	NetworkClass            int    `db:"network_class"`
	ChainID                 int    `db:"chain_id"`
	ToAddress               string `db:"to_address"`
	TokenStandard           string `db:"token_standard"`
	TokenAddress            string `db:"token_address"`
	Amount                  int64  `db:"amount"`
	Fee                     int64  `db:"fee"`
	Signature               string `db:"signature"`
	CreationMomentumHeight  int64  `db:"creation_momentum_height"`
	ConfirmationsToFinality int    `db:"confirmations_to_finality"`
}

// UnwrapTokenRequest represents a request to unwrap tokens from an external chain to Zenon
type UnwrapTokenRequest struct {
	TransactionHash            string `db:"transaction_hash"`
	LogIndex                   int64  `db:"log_index"`
	NetworkClass               int    `db:"network_class"`
	ChainID                    int    `db:"chain_id"`
	ToAddress                  string `db:"to_address"`
	TokenStandard              string `db:"token_standard"`
	TokenAddress               string `db:"token_address"`
	Amount                     int64  `db:"amount"`
	Signature                  string `db:"signature"`
	RegistrationMomentumHeight int64  `db:"registration_momentum_height"`
	Redeemed                   bool   `db:"redeemed"`
	Revoked                    bool   `db:"revoked"`
	RedeemableIn               int64  `db:"redeemable_in"`
}
