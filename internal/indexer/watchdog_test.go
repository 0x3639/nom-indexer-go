package indexer

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestClassify(t *testing.T) {
	cfg := classifyConfig{
		StallThreshold:        60 * time.Second,
		IndexerDriftThreshold: 3,
		NodeDriftThreshold:    3,
	}
	now := time.Unix(2000, 0)

	cases := []struct {
		name           string
		probe          ProbeResult
		probeErr       error
		dbHeight       int64
		lastProgressAt time.Time
		want           syncClass
	}{
		{
			name:           "probe failure wins over everything",
			probe:          ProbeResult{},
			probeErr:       errors.New("nope"),
			dbHeight:       100,
			lastProgressAt: now,
			want:           classProbeFailed,
		},
		{
			name:           "stalled wins over node_lagging",
			probe:          ProbeResult{Frontier: 200, Target: 300},
			probeErr:       nil,
			dbHeight:       100,
			lastProgressAt: now.Add(-2 * time.Minute),
			want:           classStalled,
		},
		{
			name:           "node_lagging wins over indexer_lagging",
			probe:          ProbeResult{Frontier: 100, Target: 200},
			probeErr:       nil,
			dbHeight:       50,
			lastProgressAt: now,
			want:           classNodeLagging,
		},
		{
			name:           "indexer_lagging",
			probe:          ProbeResult{Frontier: 100, Target: 100},
			probeErr:       nil,
			dbHeight:       50,
			lastProgressAt: now,
			want:           classIndexerLagging,
		},
		{
			name:           "synced exact",
			probe:          ProbeResult{Frontier: 100, Target: 100},
			probeErr:       nil,
			dbHeight:       100,
			lastProgressAt: now,
			want:           classSynced,
		},
		{
			name:           "synced with 1-momentum drift (under indexer threshold)",
			probe:          ProbeResult{Frontier: 100, Target: 100},
			probeErr:       nil,
			dbHeight:       99,
			lastProgressAt: now,
			want:           classSynced,
		},
		{
			name:           "synced with 3-momentum node lag (at threshold not over)",
			probe:          ProbeResult{Frontier: 100, Target: 103},
			probeErr:       nil,
			dbHeight:       100,
			lastProgressAt: now,
			want:           classSynced,
		},
		{
			name:           "node_lagging when target - frontier > threshold (4)",
			probe:          ProbeResult{Frontier: 100, Target: 104},
			probeErr:       nil,
			dbHeight:       100,
			lastProgressAt: now,
			want:           classNodeLagging,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classify(tc.probe, tc.probeErr, tc.dbHeight, tc.lastProgressAt, now, cfg)
			if got != tc.want {
				t.Fatalf("classify: got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSyncClassString(t *testing.T) {
	cases := map[syncClass]string{
		classSynced:         "synced",
		classIndexerLagging: "indexer_lagging",
		classNodeLagging:    "node_lagging",
		classStalled:        "stalled",
		classProbeFailed:    "probe_failed",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("syncClass(%d).String() = %q, want %q", c, got, want)
		}
	}
}

func TestReactSyncedResetsUnhealthyIncrementsHealthy(t *testing.T) {
	state := newSyncState(2)
	state.streaks[0] = nodeStreaks{unhealthy: 5}
	cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

	intent := react(state, 0, classSynced, cfg)
	if intent.signalRestart || intent.failoverIdx != -1 || intent.failbackIdx != -1 {
		t.Fatalf("synced should produce no intent on primary: %+v", intent)
	}
	if state.streaks[0].unhealthy != 0 {
		t.Fatalf("expected unhealthy reset, got %d", state.streaks[0].unhealthy)
	}
	if state.streaks[0].healthy != 1 {
		t.Fatalf("expected healthy=1, got %d", state.streaks[0].healthy)
	}
}

func TestReactIndexerLaggingSignalsRestartOnlyNoStreakTouch(t *testing.T) {
	state := newSyncState(2)
	state.streaks[0] = nodeStreaks{healthy: 3, unhealthy: 0}
	cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

	intent := react(state, 0, classIndexerLagging, cfg)
	if !intent.signalRestart {
		t.Fatal("expected restart signal")
	}
	if intent.failoverIdx != -1 {
		t.Fatalf("expected no failover, got idx %d", intent.failoverIdx)
	}
	if state.streaks[0].healthy != 3 || state.streaks[0].unhealthy != 0 {
		t.Fatalf("indexer_lagging should not touch streaks, got %+v", state.streaks[0])
	}
}

func TestReactNodeLaggingFailoverAfterStreak(t *testing.T) {
	state := newSyncState(2)
	cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

	// first bad tick → streak=1, no failover yet
	intent := react(state, 0, classNodeLagging, cfg)
	if intent.failoverIdx != -1 {
		t.Fatalf("expected no failover at streak=1, got %d", intent.failoverIdx)
	}
	if state.streaks[0].unhealthy != 1 {
		t.Fatalf("expected streak=1, got %d", state.streaks[0].unhealthy)
	}

	// second bad tick → streak=2, failover signalled
	intent = react(state, 0, classNodeLagging, cfg)
	if intent.failoverIdx == -1 {
		t.Fatal("expected failover at streak=2")
	}
	if state.streaks[0].unhealthy != 2 {
		t.Fatalf("expected streak=2, got %d", state.streaks[0].unhealthy)
	}
}

func TestReactStalledTriggersRestartEveryTickAndStreaks(t *testing.T) {
	state := newSyncState(2)
	cfg := watchdogReactConfig{UnhealthyStreak: 3, FailbackStreak: 5}

	intent := react(state, 0, classStalled, cfg)
	if !intent.signalRestart {
		t.Fatal("expected restart signal on first stalled tick")
	}
	if state.streaks[0].unhealthy != 1 {
		t.Fatalf("expected unhealthy=1, got %d", state.streaks[0].unhealthy)
	}
	if intent.failoverIdx != -1 {
		t.Fatalf("expected no failover at streak=1, got %d", intent.failoverIdx)
	}

	// Two more stalled ticks → restart still signalled each time, failover at streak=3
	react(state, 0, classStalled, cfg)
	intent = react(state, 0, classStalled, cfg)
	if !intent.signalRestart {
		t.Fatal("expected restart still signalled on third stalled tick")
	}
	if intent.failoverIdx == -1 {
		t.Fatal("expected failover at streak=3")
	}
}

func TestReactProbeFailedNoRestartFailoverAfterStreak(t *testing.T) {
	state := newSyncState(2)
	cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

	intent := react(state, 0, classProbeFailed, cfg)
	if intent.signalRestart {
		t.Fatal("probe_failed should not signal restart (we cannot trust the probe)")
	}
	if intent.failoverIdx != -1 {
		t.Fatalf("expected no failover at streak=1, got %d", intent.failoverIdx)
	}

	intent = react(state, 0, classProbeFailed, cfg)
	if intent.failoverIdx == -1 {
		t.Fatal("expected failover at streak=2")
	}
}

func TestReactSyncedFailbackIdxAlwaysMinusOne(t *testing.T) {
	// react() doesn't decide failback; that's selectFailback in Task 11.
	state := newSyncState(3)
	state.activeIdx = 2
	cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 3}

	intent := react(state, 2, classSynced, cfg)
	if intent.failbackIdx != -1 {
		t.Fatalf("react should never set failbackIdx, got %d", intent.failbackIdx)
	}
}

func TestNewSyncStateInitsStreaksForEachNode(t *testing.T) {
	s := newSyncState(3)
	if len(s.streaks) != 3 {
		t.Fatalf("expected 3 streak entries, got %d", len(s.streaks))
	}
	for i := 0; i < 3; i++ {
		if st, ok := s.streaks[i]; !ok || st != (nodeStreaks{}) {
			t.Errorf("node %d streak should be zero-valued, got %+v ok=%v", i, st, ok)
		}
	}
}

// Helper for tests below — a node that's "healthy" reports synced
// targetHeight == currentHeight at the given height. Genesis G.
func okNode(t *testing.T, height uint64, genesis string) *httptest.Server {
	t.Helper()
	return fakeJSONRPC(t, map[string]any{
		"stats.syncInfo":             map[string]any{"state": 2, "currentHeight": height, "targetHeight": height},
		"ledger.getFrontierMomentum": map[string]any{"height": height},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"hash": genesis}},
		},
	})
}

func laggingNode(t *testing.T, frontier, target uint64, genesis string) *httptest.Server {
	t.Helper()
	return fakeJSONRPC(t, map[string]any{
		"stats.syncInfo":             map[string]any{"state": 1, "currentHeight": frontier, "targetHeight": target},
		"ledger.getFrontierMomentum": map[string]any{"height": frontier},
		"ledger.getMomentumsByHeight": map[string]any{
			"count": 1,
			"list":  []any{map[string]any{"hash": genesis}},
		},
	})
}

func TestSelectFailoverPicksFirstHealthy(t *testing.T) {
	badSrv := laggingNode(t, 100, 200, "G") // node_lagging: target far ahead of frontier
	goodSrv := okNode(t, 100, "G")

	pool := NewNodePool([]NodeEntry{
		{URL: "http://primary-dead", Label: "primary"},
		{URL: badSrv.URL, Label: "bad-fb"},
		{URL: goodSrv.URL, Label: "good-fb"},
	}, zap.NewNop())

	cfg := classifyConfig{NodeDriftThreshold: 3}
	idx := selectFailoverTarget(context.Background(), pool, 0, "G", cfg)
	if idx != 2 {
		t.Fatalf("expected idx 2, got %d", idx)
	}
}

func TestSelectFailoverSkipsChainMismatch(t *testing.T) {
	wrongChain := okNode(t, 100, "X")
	rightChain := okNode(t, 100, "G")
	pool := NewNodePool([]NodeEntry{
		{URL: "http://primary", Label: "primary"},
		{URL: wrongChain.URL, Label: "wrong-chain"},
		{URL: rightChain.URL, Label: "right-chain"},
	}, zap.NewNop())

	cfg := classifyConfig{NodeDriftThreshold: 3}
	idx := selectFailoverTarget(context.Background(), pool, 0, "G", cfg)
	if idx != 2 {
		t.Fatalf("expected idx 2 (chain G), got %d", idx)
	}
}

func TestSelectFailoverReturnsMinusOneWhenAllFail(t *testing.T) {
	badSrv := laggingNode(t, 100, 200, "G")
	pool := NewNodePool([]NodeEntry{
		{URL: "http://primary", Label: "primary"},
		{URL: "http://127.0.0.1:1", Label: "dead"},
		{URL: badSrv.URL, Label: "lagging"},
	}, zap.NewNop())

	cfg := classifyConfig{NodeDriftThreshold: 3}
	if idx := selectFailoverTarget(context.Background(), pool, 0, "G", cfg); idx != -1 {
		t.Fatalf("expected -1, got %d", idx)
	}
}

func TestSelectFailoverAcceptsFirstRunEmptyGenesis(t *testing.T) {
	s := okNode(t, 100, "G") // any genesis is fine when storedGenesis is ""
	pool := NewNodePool([]NodeEntry{
		{URL: "http://primary", Label: "primary"},
		{URL: s.URL, Label: "first-ok"},
	}, zap.NewNop())
	cfg := classifyConfig{NodeDriftThreshold: 3}
	idx := selectFailoverTarget(context.Background(), pool, 0, "", cfg)
	if idx != 1 {
		t.Fatalf("expected idx 1, got %d", idx)
	}
}

func TestSelectFailbackRequiresStreak(t *testing.T) {
	state := newSyncState(2)
	state.activeIdx = 1
	cfg := watchdogReactConfig{FailbackStreak: 3}

	// 1st healthy primary probe — streak advances to 1
	if selectFailback(state, 0, cfg) != -1 {
		t.Fatal("should not failback at streak 1")
	}
	if selectFailback(state, 0, cfg) != -1 {
		t.Fatal("should not failback at streak 2")
	}
	if got := selectFailback(state, 0, cfg); got != 0 {
		t.Fatalf("should failback at streak 3, got %d", got)
	}
}

func TestSelectFailbackPreservesHealthyAfterCrossingThreshold(t *testing.T) {
	// After selectFailback returns the idx, the watchdog will swap and
	// reset all streaks. We don't reset here — the caller's responsibility.
	// Verify the streak isn't reset by selectFailback itself.
	state := newSyncState(2)
	cfg := watchdogReactConfig{FailbackStreak: 2}

	selectFailback(state, 0, cfg)
	selectFailback(state, 0, cfg)
	if state.streaks[0].healthy != 2 {
		t.Fatalf("expected healthy=2 after threshold-crossing, got %d", state.streaks[0].healthy)
	}
}
