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
	// Flow metrics — populated incrementally as blocks are processed.
	GenesisZnnBalance int64  `db:"genesis_znn_balance"`
	GenesisQsrBalance int64  `db:"genesis_qsr_balance"`
	ZnnSent           int64  `db:"znn_sent"`
	ZnnReceived       int64  `db:"znn_received"`
	QsrSent           int64  `db:"qsr_sent"`
	QsrReceived       int64  `db:"qsr_received"`
	FirstActiveAt     *int64 `db:"first_active_at"`
	LastActiveAt      *int64 `db:"last_active_at"`
	// Per-account block counters maintained from account_blocks where the
	// address appears as sender OR recipient (to_address). Distinct from
	// BlockCount (sender-only chain height) and FirstActiveAt/LastActiveAt
	// (chain owner only).
	FirstSeen *int64 `db:"first_seen"`
	LastSeen  *int64 `db:"last_seen"`
	TxCount   int64  `db:"tx_count"`
}

// Balance represents a token balance for an account
type Balance struct {
	Address              string `db:"address"`
	TokenStandard        string `db:"token_standard"`
	Balance              int64  `db:"balance"`
	LastUpdatedTimestamp int64  `db:"last_updated_timestamp"`
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

// TokenMint is a single mint event on a token.
type TokenMint struct {
	ID                int64  `db:"id"`
	AccountBlockHash  string `db:"account_block_hash"`
	MomentumHeight    int64  `db:"momentum_height"`
	MomentumTimestamp int64  `db:"momentum_timestamp"`
	TokenStandard     string `db:"token_standard"`
	Issuer            string `db:"issuer"`
	Receiver          string `db:"receiver"`
	Amount            int64  `db:"amount"`
}

// TokenBurn is a single burn event on a token.
type TokenBurn struct {
	ID                int64  `db:"id"`
	AccountBlockHash  string `db:"account_block_hash"`
	MomentumHeight    int64  `db:"momentum_height"`
	MomentumTimestamp int64  `db:"momentum_timestamp"`
	TokenStandard     string `db:"token_standard"`
	Burner            string `db:"burner"`
	Amount            int64  `db:"amount"`
}

// BridgeNetwork is one configured destination network on the Zenon bridge.
type BridgeNetwork struct {
	NetworkClass         int    `db:"network_class"`
	ChainID              int    `db:"chain_id"`
	Name                 string `db:"name"`
	ContractAddress      string `db:"contract_address"`
	Metadata             string `db:"metadata"`
	LastUpdatedTimestamp int64  `db:"last_updated_timestamp"`
}

// BridgeNetworkToken pairs a Zenon ZTS with a remote-chain token + config.
type BridgeNetworkToken struct {
	NetworkClass  int    `db:"network_class"`
	ChainID       int    `db:"chain_id"`
	TokenStandard string `db:"token_standard"`
	TokenAddress  string `db:"token_address"`
	Bridgeable    bool   `db:"bridgeable"`
	Redeemable    bool   `db:"redeemable"`
	Owned         bool   `db:"owned"`
	MinAmount     int64  `db:"min_amount"`
	FeePercentage int    `db:"fee_percentage"`
	RedeemDelay   int    `db:"redeem_delay"`
	Metadata      string `db:"metadata"`
}

// BridgeAdmin is a singleton row (row_id=1) describing the active administrator
// and bridge-wide flags pulled from BridgeApi.GetBridgeInfo.
type BridgeAdmin struct {
	Administrator              string `db:"administrator"`
	CompressedTssECDSAPubKey   string `db:"compressed_tss_ecdsa_pubkey"`
	DecompressedTssECDSAPubKey string `db:"decompressed_tss_ecdsa_pubkey"`
	AllowKeyGen                bool   `db:"allow_key_gen"`
	Halted                     bool   `db:"halted"`
	UnhaltedAt                 int64  `db:"unhalted_at"`
	UnhaltDurationInMomentums  int64  `db:"unhalt_duration_in_momentums"`
	TssNonce                   int64  `db:"tss_nonce"`
	Metadata                   string `db:"metadata"`
	LastUpdatedTimestamp       int64  `db:"last_updated_timestamp"`
}

// BridgeGuardian is one entry on the guardian set pulled from SecurityInfo.
type BridgeGuardian struct {
	Address              string `db:"address"`
	Nominated            bool   `db:"nominated"`
	Accepted             bool   `db:"accepted"`
	LastUpdatedTimestamp int64  `db:"last_updated_timestamp"`
}

// BridgeOrchestratorInfo mirrors OrchestratorInfo from the SDK (singleton).
type BridgeOrchestratorInfo struct {
	WindowSize              int64 `db:"window_size"`
	KeyGenThreshold         int   `db:"key_gen_threshold"`
	ConfirmationsToFinality int   `db:"confirmations_to_finality"`
	EstimatedMomentumTime   int   `db:"estimated_momentum_time"`
	AllowKeyGenHeight       int64 `db:"allow_key_gen_height"`
	LastUpdatedTimestamp    int64 `db:"last_updated_timestamp"`
}

// BridgeSecurityInfo mirrors SecurityInfo's delay fields (singleton).
type BridgeSecurityInfo struct {
	AdministratorDelay   int64 `db:"administrator_delay"`
	SoftDelay            int64 `db:"soft_delay"`
	LastUpdatedTimestamp int64 `db:"last_updated_timestamp"`
}

// NetworkStatHistory is a daily network-wide snapshot row.
type NetworkStatHistory struct {
	Date            string `db:"date"`
	TotalTx         int64  `db:"total_tx"`
	DailyTx         int64  `db:"daily_tx"`
	TotalAddresses  int64  `db:"total_addresses"`
	DailyAddresses  int64  `db:"daily_addresses"`
	ActiveAddresses int64  `db:"active_addresses"`
	TotalTokens     int64  `db:"total_tokens"`
	DailyTokens     int64  `db:"daily_tokens"`
	TotalStakes     int64  `db:"total_stakes"`
	DailyStakes     int64  `db:"daily_stakes"`
	TotalFusions    int64  `db:"total_fusions"`
	DailyFusions    int64  `db:"daily_fusions"`
	TotalPillars    int64  `db:"total_pillars"`
	TotalSentinels  int64  `db:"total_sentinels"`
}

// TokenStatHistory is a daily per-token snapshot row.
type TokenStatHistory struct {
	Date              string `db:"date"`
	TokenStandard     string `db:"token_standard"`
	DailyMinted       int64  `db:"daily_minted"`
	DailyBurned       int64  `db:"daily_burned"`
	TotalSupply       int64  `db:"total_supply"`
	TotalHolders      int64  `db:"total_holders"`
	TotalTransactions int64  `db:"total_transactions"`
}

// PillarStatHistory is a daily per-pillar snapshot row.
type PillarStatHistory struct {
	Date               string `db:"date"`
	PillarOwnerAddress string `db:"pillar_owner_address"`
	Rank               int    `db:"rank"`
	Weight             int64  `db:"weight"`
	MomentumRewards    int64  `db:"momentum_rewards"`
	DelegateRewards    int64  `db:"delegate_rewards"`
	TotalDelegators    int64  `db:"total_delegators"`
}

// BridgeStatHistory is a daily per-(network, token) bridge snapshot row.
type BridgeStatHistory struct {
	Date            string `db:"date"`
	NetworkClass    int    `db:"network_class"`
	ChainID         int    `db:"chain_id"`
	TokenStandard   string `db:"token_standard"`
	WrapTxCount     int64  `db:"wrap_tx_count"`
	WrappedAmount   int64  `db:"wrapped_amount"`
	UnwrapTxCount   int64  `db:"unwrap_tx_count"`
	UnwrappedAmount int64  `db:"unwrapped_amount"`
	TotalVolume     int64  `db:"total_volume"`
}

// Delegation is one interval of an account delegating to a pillar.
// EndedAt is nil for the currently-active delegation.
type Delegation struct {
	ID                 int64  `db:"id"`
	DelegatorAddress   string `db:"delegator_address"`
	PillarOwnerAddress string `db:"pillar_owner_address"`
	StartedAt          int64  `db:"started_at"`
	EndedAt            *int64 `db:"ended_at"`
}

// SyncStatus is the in-DB projection of the watchdog's last tick.
// The row is single-row (id=1) — see migrations/013.
type SyncStatus struct {
	DBHeight             int64  `db:"db_height"`
	ZnndFrontierHeight   int64  `db:"znnd_frontier_height"`
	ZnndTargetHeight     int64  `db:"znnd_target_height"`
	DriftMomentums       int64  `db:"drift_momentums"`
	NodeLagMomentums     int64  `db:"node_lag_momentums"`
	State                string `db:"state"` // synced | indexer_lagging | node_lagging | stalled | probe_failed
	ConsecutiveBadChecks int    `db:"consecutive_bad_checks"`
	ActiveNodeURL        string `db:"active_node_url"`
	ActiveNodeLabel      string `db:"active_node_label"`
	ChainIdentifier      string `db:"chain_identifier"`
	FailedOverAt         *int64 `db:"failed_over_at"`
	LastProgressAt       int64  `db:"last_progress_at"`
	CheckedAt            int64  `db:"checked_at"`
}

// HtlcStatus represents the lifecycle state of an HTLC entry.
type HtlcStatus int16

const (
	HtlcStatusActive    HtlcStatus = 0
	HtlcStatusUnlocked  HtlcStatus = 1
	HtlcStatusReclaimed HtlcStatus = 2
)

// Htlc represents a hash-time-locked contract entry.
type Htlc struct {
	ID                        string `db:"id"`
	TimeLockedAddress         string `db:"time_locked_address"`
	HashLockedAddress         string `db:"hash_locked_address"`
	TokenStandard             string `db:"token_standard"`
	Amount                    int64  `db:"amount"`
	ExpirationTimestamp       int64  `db:"expiration_timestamp"`
	HashType                  int16  `db:"hash_type"`
	KeyMaxSize                int16  `db:"key_max_size"`
	HashLock                  string `db:"hash_lock"`
	Status                    int16  `db:"status"`
	Preimage                  string `db:"preimage"`
	CreationMomentumHeight    int64  `db:"creation_momentum_height"`
	CreationMomentumTimestamp int64  `db:"creation_momentum_timestamp"`
	SettleMomentumHeight      int64  `db:"settle_momentum_height"`
	SettleMomentumTimestamp   int64  `db:"settle_momentum_timestamp"`
}
