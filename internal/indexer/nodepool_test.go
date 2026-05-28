package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// fakeJSONRPC stands up an httptest server that returns the supplied
// per-method responses. Used by both nodepool and client_switch tests.
func fakeJSONRPC(t *testing.T, responses map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]any{"jsonrpc": "2.0", "id": 1}
		if result, ok := responses[req.Method]; ok {
			resp["result"] = result
		} else {
			resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNodePoolProbeSynced(t *testing.T) {
	srv := fakeJSONRPC(t, map[string]any{
		"stats.syncInfo": map[string]any{
			"state": 2, "currentHeight": 100, "targetHeight": 100,
		},
		"ledger.getFrontierMomentum": map[string]any{"height": 100},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"height": 1, "hash": "genesis-hash"}},
		},
	})

	pool := NewNodePool([]NodeEntry{{URL: srv.URL, Label: "test"}}, zap.NewNop())
	result, err := pool.Probe(context.Background(), 0)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Frontier != 100 || result.Target != 100 {
		t.Fatalf("frontier/target: %d/%d", result.Frontier, result.Target)
	}
	if result.GenesisHash != "genesis-hash" {
		t.Fatalf("genesis hash: %q", result.GenesisHash)
	}
	if result.State != 2 {
		t.Fatalf("state: %d", result.State)
	}
}

func TestNodePoolProbeUnreachable(t *testing.T) {
	pool := NewNodePool([]NodeEntry{{URL: "http://127.0.0.1:1", Label: "dead"}}, zap.NewNop())
	_, err := pool.Probe(context.Background(), 0)
	if err == nil {
		t.Fatal("expected probe failure")
	}
}

func TestNodePoolGenesisHashCached(t *testing.T) {
	// Counter for getMomentumsByHeight calls. The first probe should
	// call it; subsequent probes should use the cached value.
	var genesisCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]any{"jsonrpc": "2.0", "id": 1}
		switch req.Method {
		case "stats.syncInfo":
			resp["result"] = map[string]any{"state": 2, "currentHeight": 50, "targetHeight": 50}
		case "ledger.getFrontierMomentum":
			resp["result"] = map[string]any{"height": 50}
		case "ledger.getMomentumsByHeight":
			genesisCalls++
			resp["result"] = map[string]any{
				"count": 1,
				"list":  []any{map[string]any{"height": 1, "hash": "g"}},
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	pool := NewNodePool([]NodeEntry{{URL: srv.URL, Label: "x"}}, zap.NewNop())
	for i := 0; i < 3; i++ {
		if _, err := pool.Probe(context.Background(), 0); err != nil {
			t.Fatalf("probe %d: %v", i, err)
		}
	}
	if genesisCalls != 1 {
		t.Fatalf("genesis fetched %d times, want 1 (cached)", genesisCalls)
	}
}

func TestNodePoolProbeOutOfRange(t *testing.T) {
	pool := NewNodePool([]NodeEntry{{URL: "http://x", Label: "x"}}, zap.NewNop())
	if _, err := pool.Probe(context.Background(), 1); err == nil {
		t.Fatal("expected error for out-of-range idx")
	}
	if _, err := pool.Probe(context.Background(), -1); err == nil {
		t.Fatal("expected error for negative idx")
	}
}
