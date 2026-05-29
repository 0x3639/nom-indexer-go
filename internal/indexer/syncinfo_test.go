package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncInfoHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"state":         2,
				"currentHeight": 13366681,
				"targetHeight":  13366681,
			},
		})
	}))
	defer srv.Close()

	info, err := fetchSyncInfo(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchSyncInfo: %v", err)
	}
	if info.State != 2 || info.CurrentHeight != 13366681 || info.TargetHeight != 13366681 {
		t.Fatalf("unexpected sync info: %#v", info)
	}
	if !info.IsSynced() {
		t.Fatal("expected IsSynced() to be true when state=2")
	}
}

func TestSyncInfoRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]any{
				"code":    -32601,
				"message": "method not found",
			},
		})
	}))
	defer srv.Close()

	_, err := fetchSyncInfo(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error on RPC error response")
	}
}

func TestSyncInfoWsURLRewrittenToHTTP(t *testing.T) {
	// Server takes plain HTTP requests but the caller passes ws:// — verify
	// the URL is rewritten before the POST goes out.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"state": 1, "currentHeight": 1, "targetHeight": 100},
		})
	}))
	defer srv.Close()

	// Convert http://127.0.0.1:N to ws://127.0.0.1:N — caller will rewrite back.
	wsURL := "ws://" + srv.Listener.Addr().String()
	info, err := fetchSyncInfo(context.Background(), wsURL)
	if err != nil {
		t.Fatalf("fetchSyncInfo: %v", err)
	}
	if info.IsSynced() {
		t.Fatal("state=1 should not be synced")
	}
}
