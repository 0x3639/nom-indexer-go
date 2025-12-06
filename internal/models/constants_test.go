package models

import "testing"

func TestIsEmbeddedContract(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{"PlasmaAddress", PlasmaAddress, true},
		{"PillarAddress", PillarAddress, true},
		{"TokenAddress", TokenAddress, true},
		{"SentinelAddress", SentinelAddress, true},
		{"StakeAddress", StakeAddress, true},
		{"AcceleratorAddress", AcceleratorAddress, true},
		{"SwapAddress", SwapAddress, true},
		{"LiquidityAddress", LiquidityAddress, true},
		{"BridgeAddress", BridgeAddress, true},
		{"HtlcAddress", HtlcAddress, true},
		{"SporkAddress", SporkAddress, true},
		{"EmptyAddress", EmptyAddress, false},
		{"random address", "z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmbeddedContract(tt.address)
			if result != tt.expected {
				t.Errorf("IsEmbeddedContract(%q) = %v, want %v", tt.address, result, tt.expected)
			}
		})
	}
}

func TestEmbeddedContractAddresses(t *testing.T) {
	addresses := EmbeddedContractAddresses()

	// Should return 11 addresses
	if len(addresses) != 11 {
		t.Errorf("EmbeddedContractAddresses() returned %d addresses, want 11", len(addresses))
	}

	// Verify all expected addresses are present
	expectedAddresses := map[string]bool{
		PlasmaAddress:      false,
		PillarAddress:      false,
		TokenAddress:       false,
		SentinelAddress:    false,
		StakeAddress:       false,
		AcceleratorAddress: false,
		SwapAddress:        false,
		LiquidityAddress:   false,
		BridgeAddress:      false,
		HtlcAddress:        false,
		SporkAddress:       false,
	}

	for _, addr := range addresses {
		if _, exists := expectedAddresses[addr]; exists {
			expectedAddresses[addr] = true
		} else {
			t.Errorf("unexpected address in EmbeddedContractAddresses: %s", addr)
		}
	}

	for addr, found := range expectedAddresses {
		if !found {
			t.Errorf("expected address not found in EmbeddedContractAddresses: %s", addr)
		}
	}
}

func TestRewardContractAddresses(t *testing.T) {
	addresses := RewardContractAddresses()

	// Should return 4 addresses
	if len(addresses) != 4 {
		t.Errorf("RewardContractAddresses() returned %d addresses, want 4", len(addresses))
	}

	// Verify all expected addresses are present
	expectedAddresses := map[string]bool{
		PillarAddress:    false,
		SentinelAddress:  false,
		StakeAddress:     false,
		LiquidityAddress: false,
	}

	for _, addr := range addresses {
		if _, exists := expectedAddresses[addr]; exists {
			expectedAddresses[addr] = true
		} else {
			t.Errorf("unexpected address in RewardContractAddresses: %s", addr)
		}
	}

	for addr, found := range expectedAddresses {
		if !found {
			t.Errorf("expected address not found in RewardContractAddresses: %s", addr)
		}
	}
}

func TestTokenStandardConstants(t *testing.T) {
	// Verify token standards are properly defined
	if ZnnTokenStandard == "" {
		t.Error("ZnnTokenStandard should not be empty")
	}
	if QsrTokenStandard == "" {
		t.Error("QsrTokenStandard should not be empty")
	}
	if EmptyTokenStandard == "" {
		t.Error("EmptyTokenStandard should not be empty")
	}

	// Verify they are different
	if ZnnTokenStandard == QsrTokenStandard {
		t.Error("ZnnTokenStandard and QsrTokenStandard should be different")
	}
}

func TestGenesisMomentumTime(t *testing.T) {
	// Genesis momentum time should be a valid Unix timestamp (Nov 24, 2021)
	if GenesisMomentumTime <= 0 {
		t.Error("GenesisMomentumTime should be a positive Unix timestamp")
	}

	// Should be after year 2020 (timestamp 1577836800) and before 2025 (timestamp 1735689600)
	if GenesisMomentumTime < 1577836800 || GenesisMomentumTime > 1735689600 {
		t.Errorf("GenesisMomentumTime %d seems out of expected range", GenesisMomentumTime)
	}
}

func TestFusionExpirationTime(t *testing.T) {
	// Fusion expiration should be 1 hour = 3600 seconds
	if FusionExpirationTime != 3600 {
		t.Errorf("FusionExpirationTime = %d, want 3600", FusionExpirationTime)
	}
}
