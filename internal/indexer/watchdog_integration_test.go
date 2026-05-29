//go:build integration

package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// fakeNode is a JSON-RPC server whose syncInfo/frontier responses can
// be toggled at runtime via atomics.
type fakeNode struct {
	URL      string
	srv      *httptest.Server
	frontier atomic.Uint64
	target   atomic.Uint64
	genesis  string
}

func newFakeNode(t *testing.T, frontier, target uint64, genesis string) *fakeNode {
	t.Helper()
	f := &fakeNode{genesis: genesis}
	f.frontier.Store(frontier)
	f.target.Store(target)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		out := map[string]any{"jsonrpc": "2.0", "id": 1}
		switch req.Method {
		case "stats.syncInfo":
			state := 1
			if f.frontier.Load() == f.target.Load() {
				state = 2
			}
			out["result"] = map[string]any{
				"state":         state,
				"currentHeight": f.frontier.Load(),
				"targetHeight":  f.target.Load(),
			}
		case "ledger.getFrontierMomentum":
			out["result"] = map[string]any{"height": f.frontier.Load()}
		case "ledger.getMomentumsByHeight":
			out["result"] = map[string]any{
				"count": 1,
				"list":  []any{map[string]any{"height": 1, "hash": f.genesis}},
			}
		default:
			out["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	f.URL = f.srv.URL
	t.Cleanup(f.srv.Close)
	return f
}

// runTickOnly invokes runWatchdogTick directly so tests don't rely on
// the ticker's wall-clock timing. This requires the watchdog's
// internals to be package-visible (we're in package indexer).
func runTickOnly(t *testing.T, i *Indexer) {
	t.Helper()
	classifyCfg := classifyConfig{
		StallThreshold:        i.watchdogCfg.StallThreshold,
		IndexerDriftThreshold: i.watchdogCfg.IndexerDriftThreshold,
		NodeDriftThreshold:    i.watchdogCfg.NodeDriftThreshold,
	}
	reactCfg := watchdogReactConfig{
		UnhealthyStreak: i.watchdogCfg.UnhealthyStreak,
		FailbackStreak:  i.watchdogCfg.FailbackStreak,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	i.runWatchdogTick(ctx, classifyCfg, reactCfg)
}

func newIntegrationIndexer(t *testing.T, nodes []*fakeNode, unhealthy, failback int) *Indexer {
	t.Helper()
	pool := newTestPool(t)
	entries := make([]NodeEntry, len(nodes))
	for i, n := range nodes {
		entries[i] = NodeEntry{URL: n.URL, Label: nodeLabel(i)}
	}
	nodePool := NewNodePool(entries, zap.NewNop())
	// Stub the client factory: the watchdog never invokes the active
	// SDK client during a tick (it talks to nodes via the pool's HTTP
	// probe path), and the SDK's real NewRpcClient would reject the
	// httptest URL (scheme must be ws/wss) and try to open a real
	// WebSocket. A zero-value RpcClient satisfies the callback-register
	// path used by swapActiveClient.
	stubFactory := func(url string) (*rpc_client.RpcClient, error) {
		return &rpc_client.RpcClient{}, nil
	}
	idx := &Indexer{
		pool:              pool,
		repos:             repository.NewRepositories(pool),
		logger:            zap.NewNop(),
		pillarNameToOwner: make(map[string]string),
		restartSubCh:      make(chan struct{}, 1),
		nodePool:          nodePool,
		syncStateInternal: newSyncState(nodePool.Len()),
		watchdogCfg: WatchdogConfigForIndexer{
			Enabled:               true,
			Interval:              200 * time.Millisecond,
			StallThreshold:        10 * time.Second,
			IndexerDriftThreshold: 3,
			NodeDriftThreshold:    3,
			UnhealthyStreak:       unhealthy,
			FailbackStreak:        failback,
		},
		clientFactory: stubFactory,
	}
	idx.lastProgressAt.Store(time.Now().Unix())
	return idx
}

func nodeLabel(i int) string {
	if i == 0 {
		return "primary"
	}
	return "fallback"
}

// --- the actual scenarios ---

func TestWatchdogFailoverOnNodeLag(t *testing.T) {
	// primary is node_lagging (target 1000 ahead of frontier 100).
	primary := newFakeNode(t, 100, 1000, "G")
	fallback := newFakeNode(t, 100, 100, "G")

	idx := newIntegrationIndexer(t, []*fakeNode{primary, fallback}, 2, 5)

	// Seed momentum row so GetLatestHeight returns >= 100.
	seedMomentumHeight(t, idx.pool, 100)

	// Tick #1 — node_lagging detected, streak=1, no failover yet.
	runTickOnly(t, idx)
	assertActive(t, idx, "primary")

	// Tick #2 — streak=2, failover to fallback.
	runTickOnly(t, idx)
	assertActive(t, idx, "fallback")
}

func TestWatchdogFailbackAfterPrimaryRecovers(t *testing.T) {
	primary := newFakeNode(t, 100, 1000, "G") // starts lagging
	fallback := newFakeNode(t, 100, 100, "G")

	idx := newIntegrationIndexer(t, []*fakeNode{primary, fallback}, 2, 3)
	seedMomentumHeight(t, idx.pool, 100)

	// Trigger failover.
	runTickOnly(t, idx)
	runTickOnly(t, idx)
	assertActive(t, idx, "fallback")

	// Primary recovers.
	primary.target.Store(100)

	// 3 healthy ticks → failback.
	runTickOnly(t, idx)
	runTickOnly(t, idx)
	runTickOnly(t, idx)
	assertActive(t, idx, "primary")
}

// --- helpers ---

func seedMomentumHeight(t *testing.T, pool *pgxpool.Pool, height int64) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		 VALUES ($1, repeat('a', 64), 1700000000, 0, 'z1qproducer', 'z1qowner', 'pillar-1')`,
		height)
	if err != nil {
		t.Fatalf("seed momentum: %v", err)
	}
}

func assertActive(t *testing.T, i *Indexer, want string) {
	t.Helper()
	i.syncStateMu.RLock()
	got := i.nodePool.Entry(i.syncStateInternal.activeIdx).Label
	i.syncStateMu.RUnlock()
	if got != want {
		t.Fatalf("active node label = %q, want %q", got, want)
	}
}
