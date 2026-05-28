package indexer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// WatchdogConfigForIndexer is the subset of internal/config.WatchdogConfig
// that the indexer package needs. Kept here so internal/indexer doesn't
// have to import internal/config (which would create a cycle once
// cmd/indexer wires both).
type WatchdogConfigForIndexer struct {
	Enabled               bool
	Interval              time.Duration
	StallThreshold        time.Duration
	IndexerDriftThreshold int64
	NodeDriftThreshold    int64
	UnhealthyStreak       int
	FailbackStreak        int
}

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

	lastClass string // last classification string (e.g. "synced"); populated by runWatchdogTick
	lastDrift int64  // frontier - dbHeight, signed (negative possible)
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

// runSyncWatchdogLoop ticks at watchdog.Interval, classifying drift and
// reacting (subscription restart, failover, failback). Returns when ctx
// is cancelled.
func (i *Indexer) runSyncWatchdogLoop(ctx context.Context) {
	ticker := time.NewTicker(i.watchdogCfg.Interval)
	defer ticker.Stop()

	classifyCfg := classifyConfig{
		StallThreshold:        i.watchdogCfg.StallThreshold,
		IndexerDriftThreshold: i.watchdogCfg.IndexerDriftThreshold,
		NodeDriftThreshold:    i.watchdogCfg.NodeDriftThreshold,
	}
	reactCfg := watchdogReactConfig{
		UnhealthyStreak: i.watchdogCfg.UnhealthyStreak,
		FailbackStreak:  i.watchdogCfg.FailbackStreak,
	}

	i.logger.Info("sync watchdog started",
		zap.Duration("interval", i.watchdogCfg.Interval),
		zap.Int("nodes", i.nodePool.Len()),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		i.runWatchdogTick(ctx, classifyCfg, reactCfg)
	}
}

// runWatchdogTick performs a single watchdog iteration: probe, classify,
// react, optionally fail over or fail back, and finally publish the
// current sync status to the database.
func (i *Indexer) runWatchdogTick(ctx context.Context, cCfg classifyConfig, rCfg watchdogReactConfig) {
	i.syncStateMu.RLock()
	activeIdx := i.syncStateInternal.activeIdx
	chainID := i.syncStateInternal.chainIdentifier
	i.syncStateMu.RUnlock()

	probe, probeErr := i.nodePool.Probe(ctx, activeIdx)

	dbHeightU, dbErr := i.repos.Momentum.GetLatestHeight(ctx)
	if dbErr != nil {
		i.logger.Warn("watchdog: GetLatestHeight failed", zap.Error(dbErr))
		return
	}
	dbHeight := int64(dbHeightU)

	lastProgress := time.Unix(i.lastProgressAt.Load(), 0)
	now := time.Now()
	class := classify(probe, probeErr, dbHeight, lastProgress, now, cCfg)

	i.syncStateMu.Lock()
	intent := react(i.syncStateInternal, activeIdx, class, rCfg)
	if probeErr == nil && i.syncStateInternal.chainIdentifier == "" {
		i.syncStateInternal.chainIdentifier = probe.GenesisHash
		chainID = probe.GenesisHash
	}
	i.syncStateInternal.lastClass = class.String()
	i.syncStateInternal.lastDrift = int64(probe.Frontier) - dbHeight
	i.syncStateMu.Unlock()

	// Failover when react() signals intent. Target chosen by
	// selectFailoverTarget (which may return -1).
	if intent.failoverIdx != -1 {
		target := selectFailoverTarget(ctx, i.nodePool, activeIdx, chainID, cCfg)
		if target == -1 {
			i.logger.Error("watchdog: no healthy fallback available",
				zap.Int("active_idx", activeIdx),
				zap.String("class", class.String()),
			)
		} else if err := i.swapActiveClient(i.nodePool.Entry(target).URL); err != nil {
			i.logger.Error("watchdog: swap failed", zap.Error(err), zap.Int("target", target))
		} else {
			i.syncStateMu.Lock()
			i.syncStateInternal.activeIdx = target
			n := now.Unix()
			i.syncStateInternal.failedOverAt = &n
			for k := range i.syncStateInternal.streaks {
				i.syncStateInternal.streaks[k] = nodeStreaks{}
			}
			i.syncStateMu.Unlock()
			i.logger.Info("watchdog: failed over",
				zap.String("from", i.nodePool.Entry(activeIdx).Label),
				zap.String("to", i.nodePool.Entry(target).Label),
			)
		}
	}

	// Failback when class is synced and we're on a fallback.
	if class == classSynced && activeIdx > 0 && intent.failoverIdx == -1 {
		for candidateIdx := 0; candidateIdx < activeIdx; candidateIdx++ {
			cProbe, err := i.nodePool.Probe(ctx, candidateIdx)
			if err != nil || cProbe.GenesisHash != chainID {
				i.syncStateMu.Lock()
				st := i.syncStateInternal.streaks[candidateIdx]
				st.healthy = 0
				i.syncStateInternal.streaks[candidateIdx] = st
				i.syncStateMu.Unlock()
				continue
			}
			i.syncStateMu.Lock()
			picked := selectFailback(i.syncStateInternal, candidateIdx, rCfg)
			i.syncStateMu.Unlock()
			if picked != -1 {
				if err := i.swapActiveClient(i.nodePool.Entry(picked).URL); err != nil {
					i.logger.Error("watchdog: failback swap failed", zap.Error(err))
					break
				}
				i.syncStateMu.Lock()
				i.syncStateInternal.activeIdx = picked
				i.syncStateInternal.failedOverAt = nil
				for k := range i.syncStateInternal.streaks {
					i.syncStateInternal.streaks[k] = nodeStreaks{}
				}
				i.syncStateMu.Unlock()
				i.logger.Info("watchdog: failed back",
					zap.String("to", i.nodePool.Entry(picked).Label),
				)
				break
			}
		}
	}

	if intent.signalRestart {
		i.signalSubscriptionRestart()
	}

	i.publishSyncStatus(ctx, probe, probeErr, dbHeight, class, activeIdx, now)
}

// publishSyncStatus writes the current watchdog state into the singleton
// indexer_sync_status row. Best-effort: a failure is logged but does not
// abort the tick (the next tick will try again).
func (i *Indexer) publishSyncStatus(
	ctx context.Context,
	probe ProbeResult,
	probeErr error,
	dbHeight int64,
	class syncClass,
	activeIdx int,
	now time.Time,
) {
	i.syncStateMu.RLock()
	st := i.syncStateInternal.streaks[activeIdx]
	chainID := i.syncStateInternal.chainIdentifier
	failedOverAt := i.syncStateInternal.failedOverAt
	i.syncStateMu.RUnlock()

	entry := i.nodePool.Entry(activeIdx)

	// When probe errored, frontier/target may be zero — record as zero,
	// log the error context once via the State field.
	record := &models.SyncStatus{
		DBHeight:             dbHeight,
		ZnndFrontierHeight:   int64(probe.Frontier),
		ZnndTargetHeight:     int64(probe.Target),
		DriftMomentums:       int64(probe.Frontier) - dbHeight,
		NodeLagMomentums:     int64(probe.Target) - int64(probe.Frontier),
		State:                class.String(),
		ConsecutiveBadChecks: st.unhealthy,
		ActiveNodeURL:        entry.URL,
		ActiveNodeLabel:      entry.Label,
		ChainIdentifier:      chainID,
		FailedOverAt:         failedOverAt,
		LastProgressAt:       i.lastProgressAt.Load(),
		CheckedAt:            now.Unix(),
	}
	if err := i.repos.SyncStatus.Upsert(ctx, record); err != nil {
		i.logger.Warn("watchdog: upsert sync_status failed", zap.Error(err))
	}
	_ = probeErr // class already encodes the failure
}
