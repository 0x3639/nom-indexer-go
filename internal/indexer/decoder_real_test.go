package indexer

import (
	"testing"

	"github.com/0x3639/znn-sdk-go/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	"go.uber.org/zap"
)

// TestTryDecodeFromAbi_Delegate exercises tryDecodeFromAbi end-to-end by
// encoding a real Pillar.Delegate call with the SDK and confirming the decoder
// recovers the method name and the "name" input.
func TestTryDecodeFromAbi_Delegate(t *testing.T) {
	encoded, err := embedded.Pillar.EncodeFunction("Delegate", []interface{}{"alphanet-1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	i := &Indexer{logger: zap.NewNop()}
	got := i.tryDecodeFromAbi(encoded, embedded.Pillar)
	if got == nil {
		t.Fatal("tryDecodeFromAbi returned nil")
	}
	if got.Method != "Delegate" {
		t.Errorf("Method = %q, want %q", got.Method, "Delegate")
	}
	if got.Inputs["name"] != "alphanet-1" {
		t.Errorf(`Inputs["name"] = %q, want "alphanet-1" (all inputs: %v)`, got.Inputs["name"], got.Inputs)
	}
}

// TestTryDecodeFromAbi_VoteByName confirms a 3-arg call with mixed types
// (hash, string, uint8) decodes correctly.
func TestTryDecodeFromAbi_VoteByName(t *testing.T) {
	hash := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	encoded, err := embedded.Accelerator.EncodeFunction("VoteByName", []interface{}{hash, "alphanet-1", uint8(0)})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	i := &Indexer{logger: zap.NewNop()}
	got := i.tryDecodeFromAbi(encoded, embedded.Accelerator)
	if got == nil {
		t.Fatal("decoder returned nil")
	}
	if got.Method != "VoteByName" {
		t.Errorf("Method = %q, want VoteByName", got.Method)
	}
	if got.Inputs["name"] != "alphanet-1" {
		t.Errorf(`Inputs["name"] = %q, want "alphanet-1"`, got.Inputs["name"])
	}
	if got.Inputs["vote"] != "0" {
		t.Errorf(`Inputs["vote"] = %q, want "0"`, got.Inputs["vote"])
	}
	// The hash input should round-trip to its hex form.
	if got.Inputs["id"] == "" {
		t.Error(`Inputs["id"] is empty, expected hex-encoded hash`)
	}
}

func TestTryDecodeFromAbi_NilAbi(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	if got := i.tryDecodeFromAbi([]byte{1, 2, 3, 4, 5}, nil); got != nil {
		t.Errorf("expected nil for nil ABI, got %+v", got)
	}
}

func TestTryDecodeFromAbi_TooShort(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	if got := i.tryDecodeFromAbi([]byte{1, 2, 3}, embedded.Pillar); got != nil {
		t.Errorf("expected nil for <4 bytes, got %+v", got)
	}
}

func TestTryDecodeFromAbi_UnknownSelector(t *testing.T) {
	i := &Indexer{logger: zap.NewNop()}
	// Random 4 bytes very unlikely to match a real selector + zero padding.
	data := []byte{0xff, 0xee, 0xdd, 0xcc, 0, 0, 0, 0}
	if got := i.tryDecodeFromAbi(data, embedded.Pillar); got != nil {
		t.Errorf("expected nil for unknown selector, got %+v", got)
	}
}
