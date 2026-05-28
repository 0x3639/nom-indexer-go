package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SyncInfo mirrors the result of stats.syncInfo as observed on the live
// znnd: {"state":N,"currentHeight":N,"targetHeight":N}. state == 2 means
// synced; smaller values mean syncing.
type SyncInfo struct {
	State         int    `json:"state"`
	CurrentHeight uint64 `json:"currentHeight"`
	TargetHeight  uint64 `json:"targetHeight"`
}

// IsSynced reports whether znnd considers itself caught up to its peers.
func (s SyncInfo) IsSynced() bool { return s.State == 2 }

// fetchSyncInfo POSTs stats.syncInfo to url and returns the parsed
// result. ws:// and wss:// inputs are transparently rewritten to
// http:// and https:// — znnd serves JSON-RPC over HTTP on the same
// host/port pair.
func fetchSyncInfo(ctx context.Context, url string) (SyncInfo, error) {
	httpURL := strings.Replace(strings.Replace(url, "wss://", "https://", 1), "ws://", "http://", 1)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "stats.syncInfo", "params": []any{},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL, bytes.NewReader(body))
	if err != nil {
		return SyncInfo{}, fmt.Errorf("syncInfo build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return SyncInfo{}, fmt.Errorf("syncInfo POST: %w", err)
	}
	defer resp.Body.Close()

	var envelope struct {
		Result *SyncInfo `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return SyncInfo{}, fmt.Errorf("syncInfo decode: %w", err)
	}
	if envelope.Error != nil {
		return SyncInfo{}, fmt.Errorf("syncInfo rpc error %d: %s",
			envelope.Error.Code, envelope.Error.Message)
	}
	if envelope.Result == nil {
		return SyncInfo{}, fmt.Errorf("syncInfo empty result")
	}
	return *envelope.Result, nil
}
