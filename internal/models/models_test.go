package models

import (
	"encoding/json"
	"testing"
)

func TestRewardType_String(t *testing.T) {
	tests := []struct {
		rt   RewardType
		want string
	}{
		{RewardTypeStake, "Stake"},
		{RewardTypeDelegation, "Delegation"},
		{RewardTypeLiquidity, "Liquidity"},
		{RewardTypeSentinel, "Sentinel"},
		{RewardTypePillar, "Pillar"},
		{RewardTypeUnknown, "Unknown"},
		{RewardType(999), "Unknown"},
		{RewardType(-1), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.rt.String(); got != tt.want {
				t.Errorf("RewardType(%d).String() = %q, want %q", tt.rt, got, tt.want)
			}
		})
	}
}

func TestNewTxData(t *testing.T) {
	td := NewTxData()
	if td == nil {
		t.Fatal("NewTxData() returned nil")
	}
	if td.Method != "" {
		t.Errorf("expected empty Method, got %q", td.Method)
	}
	if td.Inputs == nil {
		t.Fatal("Inputs should be initialized to an empty map, got nil")
	}
	if len(td.Inputs) != 0 {
		t.Errorf("expected empty Inputs, got %v", td.Inputs)
	}

	// Mutability check — caller should be able to add entries without nil deref.
	td.Inputs["foo"] = "bar"
	if td.Inputs["foo"] != "bar" {
		t.Error("Inputs map is not usable after NewTxData()")
	}
}

func TestTxData_JSONRoundTrip(t *testing.T) {
	td := NewTxData()
	td.Method = "Delegate"
	td.Inputs["name"] = "alphanet-1"

	b, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TxData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Method != "Delegate" || got.Inputs["name"] != "alphanet-1" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestSyncStatusZeroValue(t *testing.T) {
	var s SyncStatus
	if s.State != "" {
		t.Fatalf("zero value State should be empty, got %q", s.State)
	}
	if s.DBHeight != 0 {
		t.Fatalf("zero value DBHeight should be 0, got %d", s.DBHeight)
	}
}
