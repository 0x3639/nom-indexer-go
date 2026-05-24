package dto

import (
	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// Pillar is the JSON shape for a validator. Weight and SlotCostQsr ship
// as strings via Amount — they're raw int64 amounts that can exceed 2^53.
type Pillar struct {
	OwnerAddress                 string  `json:"owner_address"`
	ProducerAddress              string  `json:"producer_address"`
	WithdrawAddress              string  `json:"withdraw_address"`
	Name                         string  `json:"name"`
	Rank                         int     `json:"rank"`
	GiveMomentumRewardPercentage int16   `json:"give_momentum_reward_percentage"`
	GiveDelegateRewardPercentage int16   `json:"give_delegate_reward_percentage"`
	IsRevocable                  bool    `json:"is_revocable"`
	RevokeCooldown               int     `json:"revoke_cooldown"`
	RevokeTimestamp              int64   `json:"revoke_timestamp"`
	Weight                       Amount  `json:"weight"`
	EpochProducedMomentums       int16   `json:"epoch_produced_momentums"`
	EpochExpectedMomentums       int16   `json:"epoch_expected_momentums"`
	SlotCostQsr                  Amount  `json:"slot_cost_qsr"`
	SpawnTimestamp               int64   `json:"spawn_timestamp"`
	VotingActivity               float32 `json:"voting_activity"`
	ProducedMomentumCount        int64   `json:"produced_momentum_count"`
	IsRevoked                    bool    `json:"is_revoked"`
}

func FromPillar(p *models.Pillar) *Pillar {
	if p == nil {
		return nil
	}
	return &Pillar{
		OwnerAddress:                 p.OwnerAddress,
		ProducerAddress:              p.ProducerAddress,
		WithdrawAddress:              p.WithdrawAddress,
		Name:                         p.Name,
		Rank:                         p.Rank,
		GiveMomentumRewardPercentage: p.GiveMomentumRewardPercentage,
		GiveDelegateRewardPercentage: p.GiveDelegateRewardPercentage,
		IsRevocable:                  p.IsRevocable,
		RevokeCooldown:               p.RevokeCooldown,
		RevokeTimestamp:              p.RevokeTimestamp,
		Weight:                       AmountFromInt64(p.Weight),
		EpochProducedMomentums:       p.EpochProducedMomentums,
		EpochExpectedMomentums:       p.EpochExpectedMomentums,
		SlotCostQsr:                  AmountFromInt64(p.SlotCostQsr),
		SpawnTimestamp:               p.SpawnTimestamp,
		VotingActivity:               p.VotingActivity,
		ProducedMomentumCount:        p.ProducedMomentumCount,
		IsRevoked:                    p.IsRevoked,
	}
}

func FromPillars(in []*models.Pillar) []*Pillar {
	out := make([]*Pillar, 0, len(in))
	for _, p := range in {
		if d := FromPillar(p); d != nil {
			out = append(out, d)
		}
	}
	return out
}

// PillarDelegator is the JSON shape for /api/v1/pillars/{owner}/delegators.
type PillarDelegator struct {
	Address                  string `json:"address"`
	DelegationStartTimestamp int64  `json:"delegation_start_timestamp"`
}

func FromPillarDelegators(in []*repository.PillarDelegator) []*PillarDelegator {
	out := make([]*PillarDelegator, 0, len(in))
	for _, d := range in {
		if d == nil {
			continue
		}
		out = append(out, &PillarDelegator{
			Address:                  d.Address,
			DelegationStartTimestamp: d.DelegationStartTimestamp,
		})
	}
	return out
}
