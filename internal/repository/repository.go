package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repositories holds all repository instances
type Repositories struct {
	Momentum     *MomentumRepository
	Account      *AccountRepository
	AccountBlock *AccountBlockRepository
	Balance      *BalanceRepository
	Token        *TokenRepository
	TokenEvent   *TokenEventRepository
	Pillar       *PillarRepository
	PillarUpdate *PillarUpdateRepository
	Sentinel     *SentinelRepository
	Stake        *StakeRepository
	Fusion       *FusionRepository
	Project      *ProjectRepository
	ProjectPhase *ProjectPhaseRepository
	Vote         *VoteRepository
	Reward       *RewardRepository
	Bridge       *BridgeRepository
	BridgeConfig *BridgeConfigRepository
	Delegation   *DelegationRepository
	StatHistory  *StatHistoryRepository
	SyncStatus   *SyncStatusRepository
}

// NewRepositories creates all repository instances
func NewRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Momentum:     NewMomentumRepository(pool),
		Account:      NewAccountRepository(pool),
		AccountBlock: NewAccountBlockRepository(pool),
		Balance:      NewBalanceRepository(pool),
		Token:        NewTokenRepository(pool),
		TokenEvent:   NewTokenEventRepository(pool),
		Pillar:       NewPillarRepository(pool),
		PillarUpdate: NewPillarUpdateRepository(pool),
		Sentinel:     NewSentinelRepository(pool),
		Stake:        NewStakeRepository(pool),
		Fusion:       NewFusionRepository(pool),
		Project:      NewProjectRepository(pool),
		ProjectPhase: NewProjectPhaseRepository(pool),
		Vote:         NewVoteRepository(pool),
		Reward:       NewRewardRepository(pool),
		Bridge:       NewBridgeRepository(pool),
		BridgeConfig: NewBridgeConfigRepository(pool),
		Delegation:   NewDelegationRepository(pool),
		StatHistory:  NewStatHistoryRepository(pool),
		SyncStatus:   NewSyncStatusRepository(pool),
	}
}
