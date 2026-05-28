package indexer

import "time"

// syncClass is the result of classifying a single watchdog tick.
//
// Precedence (first match wins): probe_failed > stalled > node_lagging
// > indexer_lagging > synced.
//   - probe_failed wins because we cannot trust the other fields when
//     the probe itself errored.
//   - stalled wins over node_lagging because a stall is actionable
//     immediately (restart the subscription), whereas node_lagging is
//     something we accumulate a streak on before failing over.
//   - node_lagging wins over indexer_lagging because if znnd itself is
//     behind, the indexer-side "drift" is just us correctly waiting
//     for znnd to catch up.
type syncClass int

const (
	classSynced syncClass = iota
	classIndexerLagging
	classNodeLagging
	classStalled
	classProbeFailed
)

func (c syncClass) String() string {
	return [...]string{"synced", "indexer_lagging", "node_lagging", "stalled", "probe_failed"}[c]
}

// classifyConfig groups the per-tick thresholds. Subset of WatchdogConfig
// — only the fields classify() actually reads.
type classifyConfig struct {
	StallThreshold        time.Duration
	IndexerDriftThreshold int64
	NodeDriftThreshold    int64
}

// classify maps a single tick's observations to exactly one class.
// See syncClass for precedence rationale.
func classify(
	probe ProbeResult,
	probeErr error,
	dbHeight int64,
	lastProgressAt time.Time,
	now time.Time,
	cfg classifyConfig,
) syncClass {
	if probeErr != nil {
		return classProbeFailed
	}
	if now.Sub(lastProgressAt) > cfg.StallThreshold {
		return classStalled
	}
	if int64(probe.Target)-int64(probe.Frontier) > cfg.NodeDriftThreshold {
		return classNodeLagging
	}
	if int64(probe.Frontier)-dbHeight > cfg.IndexerDriftThreshold {
		return classIndexerLagging
	}
	return classSynced
}
