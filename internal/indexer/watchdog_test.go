package indexer

import (
	"errors"
	"testing"
	"time"
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
