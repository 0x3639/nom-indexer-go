package indexer

import (
	"context"
	"time"
)

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

// nodeStreaks tracks consecutive healthy/unhealthy ticks for one node.
type nodeStreaks struct {
	healthy   int
	unhealthy int
}

// syncState is the in-memory state owned by the watchdog goroutine. It
// is mutated only by the goroutine; reads from elsewhere (e.g. the
// indexer's /readyz handler) must use a sync.RWMutex-protected snapshot.
type syncState struct {
	activeIdx       int
	lastProgressAt  time.Time
	streaks         map[int]nodeStreaks
	chainIdentifier string // genesis hash, populated on first successful probe
	failedOverAt    *int64 // unix seconds; nil when on primary
}

// newSyncState builds a fresh state with empty streaks for each node.
func newSyncState(numNodes int) *syncState {
	s := &syncState{streaks: make(map[int]nodeStreaks, numNodes)}
	for i := 0; i < numNodes; i++ {
		s.streaks[i] = nodeStreaks{}
	}
	return s
}

// watchdogReactConfig is the subset of WatchdogConfig that react() reads.
type watchdogReactConfig struct {
	UnhealthyStreak int
	FailbackStreak  int
}

// reactIntent is the side-effect plan returned by react(). The watchdog
// goroutine reads it and issues the actual restart/swap calls. Keeping
// react() pure means we can table-test streak logic without spinning
// up goroutines or fake clients.
type reactIntent struct {
	signalRestart bool
	failoverIdx   int // -1 if no failover
	failbackIdx   int // -1 if no failback (failback selection itself happens in selectFailback)
}

// react updates s.streaks[activeIdx] for the given classification and
// returns the intent the watchdog should act on this tick.
//
// failbackIdx is always -1 here — failback requires probing other
// nodes (see selectFailback in Task 11) and so cannot be decided from
// classification alone. The watchdog goroutine layers that decision on
// top of react()'s output when c == classSynced and activeIdx > 0.
func react(s *syncState, activeIdx int, c syncClass, cfg watchdogReactConfig) reactIntent {
	intent := reactIntent{failoverIdx: -1, failbackIdx: -1}
	st := s.streaks[activeIdx]

	switch c {
	case classSynced:
		st.unhealthy = 0
		st.healthy++
		s.streaks[activeIdx] = st

	case classIndexerLagging:
		intent.signalRestart = true
		// do not touch streaks — indexer is the laggard, not the node

	case classNodeLagging:
		st.unhealthy++
		st.healthy = 0
		s.streaks[activeIdx] = st
		if st.unhealthy >= cfg.UnhealthyStreak {
			intent.failoverIdx = activeIdx
		}

	case classStalled:
		intent.signalRestart = true
		st.unhealthy++
		s.streaks[activeIdx] = st
		if st.unhealthy >= cfg.UnhealthyStreak {
			intent.failoverIdx = activeIdx
		}

	case classProbeFailed:
		st.unhealthy++
		s.streaks[activeIdx] = st
		if st.unhealthy >= cfg.UnhealthyStreak {
			intent.failoverIdx = activeIdx
		}
	}
	return intent
}

// selectFailoverTarget walks candidates starting *after* currentIdx and
// returns the first one whose probe is healthy (probe succeeds + target -
// frontier <= cfg.NodeDriftThreshold) AND whose chain matches
// storedGenesis. Returns -1 if no candidate qualifies.
//
// storedGenesis == "" means "first run, accept any chain" — the watchdog
// will record the candidate's genesis as canonical after the swap.
func selectFailoverTarget(
	ctx context.Context,
	pool *NodePool,
	currentIdx int,
	storedGenesis string,
	cfg classifyConfig,
) int {
	for idx := currentIdx + 1; idx < pool.Len(); idx++ {
		probe, err := pool.Probe(ctx, idx)
		if err != nil {
			continue
		}
		if int64(probe.Target)-int64(probe.Frontier) > cfg.NodeDriftThreshold {
			continue
		}
		if storedGenesis != "" && probe.GenesisHash != storedGenesis {
			continue
		}
		return idx
	}
	return -1
}

// selectFailback advances s.streaks[candidateIdx].healthy and returns
// candidateIdx when the streak reaches cfg.FailbackStreak. Otherwise
// returns -1. Mutates s.streaks[candidateIdx]; does NOT reset on
// threshold-crossing (the watchdog goroutine resets all streaks after
// a successful swap).
func selectFailback(s *syncState, candidateIdx int, cfg watchdogReactConfig) int {
	st := s.streaks[candidateIdx]
	st.healthy++
	s.streaks[candidateIdx] = st
	if st.healthy >= cfg.FailbackStreak {
		return candidateIdx
	}
	return -1
}
