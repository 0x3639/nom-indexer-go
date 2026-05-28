package indexer

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestSwapClientRefusesOnChainMismatch(t *testing.T) {
	candidate := fakeJSONRPC(t, map[string]any{
		"stats.syncInfo":             map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
		"ledger.getFrontierMomentum": map[string]any{"height": 100},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"hash": "genesis-B"}},
		},
	})

	pool := NewNodePool([]NodeEntry{
		{URL: "http://unused", Label: "primary"},
		{URL: candidate.URL, Label: "candidate"},
	}, zap.NewNop())

	publishCalled := false
	restartCalled := false

	err := swapClient(context.Background(), zap.NewNop(), pool, 1, "genesis-A",
		func(newURL string) error { publishCalled = true; return nil },
		func() { restartCalled = true },
	)
	if err == nil {
		t.Fatal("expected error on chain mismatch")
	}
	if publishCalled {
		t.Fatal("publish should not run when genesis mismatches")
	}
	if restartCalled {
		t.Fatal("restart should not run when genesis mismatches")
	}
}

func TestSwapClientPublishesOnMatch(t *testing.T) {
	candidate := fakeJSONRPC(t, map[string]any{
		"stats.syncInfo":             map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
		"ledger.getFrontierMomentum": map[string]any{"height": 100},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"hash": "genesis-shared"}},
		},
	})

	pool := NewNodePool([]NodeEntry{
		{URL: "http://unused", Label: "primary"},
		{URL: candidate.URL, Label: "candidate"},
	}, zap.NewNop())

	var publishedURL string
	publish := func(newURL string) error { publishedURL = newURL; return nil }
	restartCalled := false
	restart := func() { restartCalled = true }

	if err := swapClient(context.Background(), zap.NewNop(), pool, 1, "genesis-shared", publish, restart); err != nil {
		t.Fatalf("swapClient: %v", err)
	}
	if publishedURL != candidate.URL {
		t.Fatalf("published URL: %q want %q", publishedURL, candidate.URL)
	}
	if !restartCalled {
		t.Fatal("restart was not called")
	}
}

func TestSwapClientFirstRunSkipsGenesisCheck(t *testing.T) {
	candidate := fakeJSONRPC(t, map[string]any{
		"stats.syncInfo":             map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
		"ledger.getFrontierMomentum": map[string]any{"height": 100},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"hash": "genesis-X"}},
		},
	})

	pool := NewNodePool([]NodeEntry{{URL: candidate.URL, Label: "first"}}, zap.NewNop())

	publishCalled := false
	if err := swapClient(context.Background(), zap.NewNop(), pool, 0, "",
		func(newURL string) error { publishCalled = true; return nil },
		func() {},
	); err != nil {
		t.Fatalf("swapClient: %v", err)
	}
	if !publishCalled {
		t.Fatal("publish should run when storedGenesis is empty (first-run)")
	}
}

func TestSwapClientProbeFailureBlocksSwap(t *testing.T) {
	pool := NewNodePool([]NodeEntry{
		{URL: "http://primary", Label: "primary"},
		{URL: "http://127.0.0.1:1", Label: "dead-fallback"},
	}, zap.NewNop())

	publishCalled := false
	err := swapClient(context.Background(), zap.NewNop(), pool, 1, "genesis-A",
		func(newURL string) error { publishCalled = true; return nil },
		func() {},
	)
	if err == nil {
		t.Fatal("expected probe failure error")
	}
	if publishCalled {
		t.Fatal("publish should not run when probe fails")
	}
}

func TestSwapClientOutOfRange(t *testing.T) {
	pool := NewNodePool([]NodeEntry{{URL: "http://x", Label: "x"}}, zap.NewNop())
	err := swapClient(context.Background(), zap.NewNop(), pool, 5, "g",
		func(string) error { return nil },
		func() {},
	)
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
}
