package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// NodeEntry is one upstream Zenon node configured for the watchdog.
// URL accepts ws://, wss://, http://, or https://. Label appears in
// logs and the indexer_sync_status row.
//
// This is distinct from internal/config.NodeEntry (same shape, two
// packages). cmd/indexer adapts between the two with toIndexerNodes()
// in a later task; we don't import internal/config here because the
// watchdog should be testable without the full config stack.
type NodeEntry struct {
	URL   string
	Label string
}

// NodePool owns the ordered list of node entries and a cache of per-URL
// probe state. It does NOT own the active SDK client — that lives on
// the Indexer as an atomic.Pointer.
type NodePool struct {
	entries []NodeEntry
	logger  *zap.Logger

	mu          sync.Mutex
	genesisHash map[int]string // nodeIdx -> first-momentum hash
}

// NewNodePool builds a pool over the given entries.
func NewNodePool(entries []NodeEntry, logger *zap.Logger) *NodePool {
	return &NodePool{
		entries:     entries,
		logger:      logger,
		genesisHash: make(map[int]string),
	}
}

// Len reports the number of configured nodes.
func (p *NodePool) Len() int { return len(p.entries) }

// Entry returns the n-th node by index.
func (p *NodePool) Entry(idx int) NodeEntry { return p.entries[idx] }

// ProbeResult is the per-tick health snapshot of a single node.
type ProbeResult struct {
	Frontier    uint64
	Target      uint64
	State       int // 2 = synced (per stats.syncInfo)
	GenesisHash string
	Latency     time.Duration
}

// Probe issues stats.syncInfo + ledger.getFrontierMomentum + (first
// time only per node) ledger.getMomentumsByHeight(1, 1). Returns the
// first error encountered.
func (p *NodePool) Probe(ctx context.Context, idx int) (ProbeResult, error) {
	if idx < 0 || idx >= len(p.entries) {
		return ProbeResult{}, fmt.Errorf("probe: idx %d out of range (len=%d)", idx, len(p.entries))
	}
	url := p.entries[idx].URL
	start := time.Now()

	info, err := fetchSyncInfo(ctx, url)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("syncInfo: %w", err)
	}

	frontier, err := callForUint64(ctx, url, "ledger.getFrontierMomentum", []any{}, "height")
	if err != nil {
		return ProbeResult{}, fmt.Errorf("frontier: %w", err)
	}

	genesis, err := p.genesisFor(ctx, idx, url)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("genesis: %w", err)
	}

	return ProbeResult{
		Frontier:    frontier,
		Target:      info.TargetHeight,
		State:       info.State,
		GenesisHash: genesis,
		Latency:     time.Since(start),
	}, nil
}

// genesisFor returns the cached chain identifier (genesis momentum hash)
// for the given node, fetching it once.
func (p *NodePool) genesisFor(ctx context.Context, idx int, url string) (string, error) {
	p.mu.Lock()
	h, ok := p.genesisHash[idx]
	p.mu.Unlock()
	if ok {
		return h, nil
	}

	h, err := callForString(ctx, url, "ledger.getMomentumsByHeight", []any{1, 1}, "list.0.hash")
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	p.genesisHash[idx] = h
	p.mu.Unlock()
	return h, nil
}

// --- JSON-RPC helpers ---

// rpcURL rewrites ws(s):// to http(s):// for JSON-RPC over HTTP.
// Same logic as fetchSyncInfo's inline rewrite.
func rpcURL(u string) string {
	u = strings.Replace(u, "wss://", "https://", 1)
	u = strings.Replace(u, "ws://", "http://", 1)
	return u
}

// rpcCall issues a single JSON-RPC POST and returns the raw `result`.
func rpcCall(ctx context.Context, url, method string, params []any) (any, error) {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL(url), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rpcHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope struct {
		Result any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("rpc %s: code=%d msg=%s", method, envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}

// navigate walks a dotted JSON path (e.g. "list.0.hash") through nested
// map/slice values. Numeric path components index into []any.
func navigate(v any, path string) (any, error) {
	if path == "" {
		return v, nil
	}
	for _, p := range strings.Split(path, ".") {
		switch cur := v.(type) {
		case map[string]any:
			v = cur[p]
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("non-numeric slice segment %q: %w", p, err)
			}
			if idx < 0 || idx >= len(cur) {
				return nil, fmt.Errorf("index %d out of range at %q", idx, p)
			}
			v = cur[idx]
		default:
			return nil, fmt.Errorf("cannot descend %T at %q", v, p)
		}
	}
	return v, nil
}

func callForUint64(ctx context.Context, url, method string, params []any, path string) (uint64, error) {
	res, err := rpcCall(ctx, url, method, params)
	if err != nil {
		return 0, err
	}
	leaf, err := navigate(res, path)
	if err != nil {
		return 0, err
	}
	switch v := leaf.(type) {
	case float64:
		return uint64(v), nil
	case json.Number:
		n, _ := v.Int64()
		return uint64(n), nil
	default:
		return 0, fmt.Errorf("expected number at %s, got %T", path, leaf)
	}
}

func callForString(ctx context.Context, url, method string, params []any, path string) (string, error) {
	res, err := rpcCall(ctx, url, method, params)
	if err != nil {
		return "", err
	}
	leaf, err := navigate(res, path)
	if err != nil {
		return "", err
	}
	s, ok := leaf.(string)
	if !ok {
		return "", fmt.Errorf("expected string at %s, got %T", path, leaf)
	}
	return s, nil
}
