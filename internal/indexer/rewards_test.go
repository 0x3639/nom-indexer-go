package indexer

import (
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestDetermineRewardType(t *testing.T) {
	i := &Indexer{}

	tests := []struct {
		name string
		src  string
		want models.RewardType
	}{
		{"pillar contract", models.PillarAddress, models.RewardTypePillar},
		{"sentinel contract", models.SentinelAddress, models.RewardTypeSentinel},
		{"stake contract", models.StakeAddress, models.RewardTypeStake},
		{"liquidity contract", models.LiquidityAddress, models.RewardTypeLiquidity},
		{"unknown random", "z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7", models.RewardTypeUnknown},
		{"empty address", "", models.RewardTypeUnknown},
		{"plasma is not a reward source", models.PlasmaAddress, models.RewardTypeUnknown},
		{"token contract is not a reward source", models.TokenAddress, models.RewardTypeUnknown},
		{"accelerator is not a reward source", models.AcceleratorAddress, models.RewardTypeUnknown},
		{"bridge is not a reward source", models.BridgeAddress, models.RewardTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := i.determineRewardType(tt.src)
			if got != tt.want {
				t.Errorf("determineRewardType(%q) = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}
