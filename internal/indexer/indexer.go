package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0x3639/znn-sdk-go/embedded"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zenon-network/go-zenon/common/types"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// Subscription watchdog constants
const (
	// How often to check if momentums are being received
	watchdogCheckInterval = 30 * time.Second
	// How long without a momentum before triggering reconnect
	momentumStallThreshold = 60 * time.Second
	// Maximum backoff time between reconnection attempts
	maxReconnectBackoff = 5 * time.Minute
	// Initial backoff time for reconnection
	initialReconnectBackoff = 5 * time.Second
)

// Indexer handles the indexing of blockchain data
type Indexer struct {
	client *rpc_client.RpcClient
	pool   *pgxpool.Pool
	repos  *repository.Repositories
	logger *zap.Logger

	// Cached data from node
	pillars  []*models.Pillar
	pillarMu sync.RWMutex

	// Pillar name to owner address mapping
	pillarNameToOwner map[string]string

	// Heartbeat tracking for subscription health
	lastMomentumTime   time.Time
	lastMomentumTimeMu sync.RWMutex

	// Channel to signal reconnection needed
	reconnectCh chan struct{}
}

// NewIndexer creates a new indexer instance
func NewIndexer(client *rpc_client.RpcClient, pool *pgxpool.Pool, logger *zap.Logger) *Indexer {
	return &Indexer{
		client:            client,
		pool:              pool,
		repos:             repository.NewRepositories(pool),
		logger:            logger,
		pillarNameToOwner: make(map[string]string),
		lastMomentumTime:  time.Now(),
		reconnectCh:       make(chan struct{}, 1),
	}
}

// updateLastMomentumTime updates the heartbeat timestamp
func (i *Indexer) updateLastMomentumTime() {
	i.lastMomentumTimeMu.Lock()
	i.lastMomentumTime = time.Now()
	i.lastMomentumTimeMu.Unlock()
}

// getLastMomentumTime returns the last momentum timestamp
func (i *Indexer) getLastMomentumTime() time.Time {
	i.lastMomentumTimeMu.RLock()
	defer i.lastMomentumTimeMu.RUnlock()
	return i.lastMomentumTime
}

// triggerReconnect signals that a reconnection is needed
func (i *Indexer) triggerReconnect() {
	select {
	case i.reconnectCh <- struct{}{}:
	default:
		// Channel already has a pending reconnect signal
	}
}

// Run starts the indexer main loop
func (i *Indexer) Run(ctx context.Context) error {
	i.logger.Info("starting indexer")

	// Start bridge sync in separate goroutine (runs every 1 minute)
	go i.runBridgeSyncLoop(ctx, 1*time.Minute)

	// Start cached data sync in separate goroutine (runs every 5 minutes)
	go i.runCachedDataSyncLoop(ctx, 5*time.Minute)

	// Initial sync to catch up to current height
	if err := i.sync(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// Subscribe to new momentums for real-time updates
	// This runs independently - momentums are processed immediately as they arrive
	i.logger.Info("initial sync complete, starting real-time subscription")

	// Start subscription watchdog to detect stalls and trigger reconnects
	go i.runSubscriptionWatchdog(ctx)

	return i.runResilientSubscriptionLoop(ctx)
}

// sync performs the catch-up sync from last indexed height
func (i *Indexer) sync(ctx context.Context) error {
	// Update cached data from node
	if err := i.updateCachedData(ctx); err != nil {
		i.logger.Warn("failed to update cached data", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dbHeight, err := i.repos.Momentum.GetLatestHeight(ctx)
		if err != nil {
			return fmt.Errorf("failed to get latest height: %w", err)
		}

		frontier, err := i.client.LedgerApi.GetFrontierMomentum()
		if err != nil {
			return fmt.Errorf("failed to get frontier momentum: %w", err)
		}

		if dbHeight >= frontier.Height {
			i.logger.Info("sync complete", zap.Uint64("height", dbHeight))
			return nil
		}

		// Calculate start height - genesis momentum is at height 1
		var startHeight uint64
		if dbHeight == 0 {
			startHeight = 1
		} else {
			startHeight = dbHeight + 1
		}

		// Fetch and process momentums in batches
		batchSize := uint64(100)
		momentums, err := i.client.LedgerApi.GetMomentumsByHeight(startHeight, batchSize)
		if err != nil {
			return fmt.Errorf("failed to get momentums at height %d: %w", startHeight, err)
		}

		if momentums == nil || len(momentums.List) == 0 {
			i.logger.Debug("no momentums returned", zap.Uint64("startHeight", startHeight))
			time.Sleep(time.Second)
			continue
		}

		for _, m := range momentums.List {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			i.logger.Info("processing momentum",
				zap.Uint64("height", m.Height),
				zap.Int("txCount", len(m.Content)))

			if err := i.processMomentum(ctx, m); err != nil {
				return fmt.Errorf("failed to process momentum %d: %w", m.Height, err)
			}
		}

		// Update cached data periodically
		if startHeight%1000 == 0 {
			if err := i.updateCachedData(ctx); err != nil {
				i.logger.Warn("failed to update cached data", zap.Error(err))
			}
		}
	}
}

// runSubscriptionWatchdog monitors momentum reception and triggers reconnects when stalled
func (i *Indexer) runSubscriptionWatchdog(ctx context.Context) {
	i.logger.Info("starting subscription watchdog",
		zap.Duration("checkInterval", watchdogCheckInterval),
		zap.Duration("stallThreshold", momentumStallThreshold))

	ticker := time.NewTicker(watchdogCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("subscription watchdog stopped")
			return
		case <-ticker.C:
			lastTime := i.getLastMomentumTime()
			timeSinceLast := time.Since(lastTime)

			if timeSinceLast > momentumStallThreshold {
				i.logger.Warn("momentum subscription appears stalled, triggering reconnect",
					zap.Duration("timeSinceLastMomentum", timeSinceLast),
					zap.Duration("threshold", momentumStallThreshold))
				i.triggerReconnect()
			}
		}
	}
}

// runResilientSubscriptionLoop runs the WebSocket subscription with automatic reconnection
func (i *Indexer) runResilientSubscriptionLoop(ctx context.Context) error {
	backoff := initialReconnectBackoff

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Run a single subscription session
		err := i.runSingleSubscriptionSession(ctx)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			i.logger.Warn("subscription session ended with error",
				zap.Error(err),
				zap.Duration("backoff", backoff))
		} else {
			i.logger.Info("subscription session ended, reconnecting",
				zap.Duration("backoff", backoff))
		}

		// Wait before reconnecting (with backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Do a catch-up sync before reconnecting to ensure we haven't missed blocks
		i.logger.Info("performing catch-up sync before reconnecting")
		if err := i.sync(ctx); err != nil {
			i.logger.Error("catch-up sync failed", zap.Error(err))
		}

		// Increase backoff for next time (exponential with cap)
		backoff = backoff * 2
		if backoff > maxReconnectBackoff {
			backoff = maxReconnectBackoff
		}
	}
}

// runSingleSubscriptionSession runs a single WebSocket subscription session
// Returns when the session ends (either by error, channel close, or reconnect signal)
func (i *Indexer) runSingleSubscriptionSession(ctx context.Context) error {
	// Create subscription to momentums
	sub, momentumCh, err := i.client.SubscriberApi.ToMomentums(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to momentums: %w", err)
	}
	defer sub.Unsubscribe()

	i.logger.Info("subscribed to momentums")

	// Reset heartbeat and backoff on successful connection
	i.updateLastMomentumTime()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-i.reconnectCh:
			i.logger.Info("reconnect signal received, ending subscription session")
			return nil

		case momentums, ok := <-momentumCh:
			if !ok {
				return fmt.Errorf("momentum subscription channel closed")
			}

			for _, m := range momentums {
				i.logger.Info("received new momentum",
					zap.Uint64("height", m.Height))

				// Update heartbeat
				i.updateLastMomentumTime()

				// Fetch full momentum details
				fullMomentum, err := i.client.LedgerApi.GetMomentumsByHeight(m.Height, 1)
				if err != nil || fullMomentum == nil || len(fullMomentum.List) == 0 {
					i.logger.Error("failed to get momentum details",
						zap.Uint64("height", m.Height),
						zap.Error(err))
					continue
				}

				if err := i.processMomentum(ctx, fullMomentum.List[0]); err != nil {
					i.logger.Error("failed to process momentum",
						zap.Uint64("height", m.Height),
						zap.Error(err))
				}
			}
		}
	}
}

// updateCachedData updates pillar and other cached data from the node
func (i *Indexer) updateCachedData(ctx context.Context) error {
	i.logger.Info("updateCachedData: starting")

	// Update pillars
	i.logger.Info("updateCachedData: fetching pillars")
	pillarList, err := i.client.PillarApi.GetAll(0, 200)
	if err != nil {
		return fmt.Errorf("failed to get pillars: %w", err)
	}

	i.pillarMu.Lock()
	i.pillars = make([]*models.Pillar, 0, len(pillarList.List))
	i.pillarNameToOwner = make(map[string]string)

	for _, p := range pillarList.List {
		pillar := &models.Pillar{
			OwnerAddress:                 p.OwnerAddress.String(),
			ProducerAddress:              p.ProducerAddress.String(),
			WithdrawAddress:              p.WithdrawAddress.String(),
			Name:                         p.Name,
			Rank:                         int(p.Rank),
			GiveMomentumRewardPercentage: int16(p.GiveMomentumRewardPercentage),
			GiveDelegateRewardPercentage: int16(p.GiveDelegateRewardPercentage),
			IsRevocable:                  p.IsRevocable,
			RevokeCooldown:               int(p.RevokeCooldown),
			RevokeTimestamp:              p.RevokeTimestamp,
			Weight:                       p.Weight.Int64(),
			EpochProducedMomentums:       int16(p.CurrentStats.ProducedMomentums),
			EpochExpectedMomentums:       int16(p.CurrentStats.ExpectedMomentums),
		}
		i.pillars = append(i.pillars, pillar)
		i.pillarNameToOwner[p.Name] = p.OwnerAddress.String()

		// Save to database
		if err := i.repos.Pillar.Upsert(ctx, pillar); err != nil {
			i.logger.Warn("failed to upsert pillar", zap.String("name", p.Name), zap.Error(err))
		}
	}
	i.pillarMu.Unlock()

	i.logger.Info("updateCachedData: pillars done", zap.Int("count", len(pillarList.List)))

	// Update sentinels with pagination
	i.logger.Info("updateCachedData: fetching sentinels")
	sentinelCount := 0
	sentinelPageIndex := uint32(0)
	sentinelPageSize := uint32(10)

	for {
		sentinelList, err := i.client.SentinelApi.GetAllActive(sentinelPageIndex, sentinelPageSize)
		if err != nil {
			i.logger.Warn("failed to get sentinels", zap.Error(err))
			break
		}

		if len(sentinelList.List) == 0 {
			break
		}

		for _, s := range sentinelList.List {
			sentinel := &models.Sentinel{
				Owner:                 s.Owner.String(),
				RegistrationTimestamp: s.RegistrationTimestamp,
				IsRevocable:           s.IsRevocable,
				RevokeCooldown:        fmt.Sprintf("%d", s.RevokeCooldown),
				Active:                s.Active,
			}
			if err := i.repos.Sentinel.Upsert(ctx, sentinel); err != nil {
				i.logger.Warn("failed to upsert sentinel", zap.String("owner", s.Owner.String()), zap.Error(err))
			}
			sentinelCount++
		}

		if len(sentinelList.List) < int(sentinelPageSize) {
			break
		}
		sentinelPageIndex++
	}

	i.logger.Info("updateCachedData: sentinels done", zap.Int("count", sentinelCount))

	// Update projects with pagination
	i.logger.Info("updateCachedData: fetching projects from accelerator")
	projectCount := 0
	phaseCount := 0
	projectPageIndex := uint32(0)
	projectPageSize := uint32(10)

	for {
		projectList, err := i.client.AcceleratorApi.GetAll(projectPageIndex, projectPageSize)
		if err != nil {
			i.logger.Warn("failed to get projects", zap.Error(err))
			break
		}

		if len(projectList.List) == 0 {
			break
		}

		for _, p := range projectList.List {
			votingID := i.getVotingID(p.Id.String())

			yesVotes := int16(0)
			noVotes := int16(0)
			totalVotes := int16(0)
			if p.Votes != nil {
				yesVotes = int16(p.Votes.Yes)
				noVotes = int16(p.Votes.No)
				totalVotes = int16(p.Votes.Total)
			}

			project := &models.Project{
				ID:                  p.Id.String(),
				VotingID:            votingID,
				Owner:               p.Owner.String(),
				Name:                p.Name,
				Description:         p.Description,
				URL:                 p.Url,
				ZnnFundsNeeded:      p.ZnnFundsNeeded.Int64(),
				QsrFundsNeeded:      p.QsrFundsNeeded.Int64(),
				CreationTimestamp:   p.CreationTimestamp,
				LastUpdateTimestamp: p.LastUpdateTimestamp,
				Status:              int16(p.Status),
				YesVotes:            yesVotes,
				NoVotes:             noVotes,
				TotalVotes:          totalVotes,
			}
			if err := i.repos.Project.Upsert(ctx, project); err != nil {
				i.logger.Warn("failed to upsert project", zap.String("id", p.Id.String()), zap.Error(err))
			}
			projectCount++

			// Update phases from in-place data
			for _, phase := range p.Phases {
				if phase.Phase == nil {
					continue
				}
				phaseVotingID := i.getVotingID(phase.Phase.Id.String())

				phaseYesVotes := int16(0)
				phaseNoVotes := int16(0)
				phaseTotalVotes := int16(0)
				if phase.Votes != nil {
					phaseYesVotes = int16(phase.Votes.Yes)
					phaseNoVotes = int16(phase.Votes.No)
					phaseTotalVotes = int16(phase.Votes.Total)
				}

				projectPhase := &models.ProjectPhase{
					ID:                phase.Phase.Id.String(),
					ProjectID:         p.Id.String(),
					VotingID:          phaseVotingID,
					Name:              phase.Phase.Name,
					Description:       phase.Phase.Description,
					URL:               phase.Phase.Url,
					ZnnFundsNeeded:    phase.Phase.ZnnFundsNeeded.Int64(),
					QsrFundsNeeded:    phase.Phase.QsrFundsNeeded.Int64(),
					CreationTimestamp: phase.Phase.CreationTimestamp,
					AcceptedTimestamp: phase.Phase.AcceptedTimestamp,
					Status:            int16(phase.Phase.Status),
					YesVotes:          phaseYesVotes,
					NoVotes:           phaseNoVotes,
					TotalVotes:        phaseTotalVotes,
				}
				if err := i.repos.ProjectPhase.Upsert(ctx, projectPhase); err != nil {
					i.logger.Warn("failed to upsert project phase", zap.String("id", phase.Phase.Id.String()), zap.Error(err))
				}
				phaseCount++
			}
		}

		if len(projectList.List) < int(projectPageSize) {
			break
		}
		projectPageIndex++
	}

	i.logger.Info("updateCachedData: projects done", zap.Int("projects", projectCount), zap.Int("phases", phaseCount))

	i.logger.Info("updateCachedData: complete")
	return nil
}

// runCachedDataSyncLoop runs cached data sync (pillars, sentinels, projects) on a separate schedule
func (i *Indexer) runCachedDataSyncLoop(ctx context.Context, interval time.Duration) {
	i.logger.Info("starting cached data sync loop", zap.Duration("interval", interval))

	// Run immediately on startup
	if err := i.updateCachedData(ctx); err != nil {
		i.logger.Warn("cached data sync: initial sync failed", zap.Error(err))
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("cached data sync loop stopped")
			return
		case <-ticker.C:
			if err := i.updateCachedData(ctx); err != nil {
				i.logger.Warn("cached data sync: failed", zap.Error(err))
			}
		}
	}
}

// runBridgeSyncLoop runs bridge data sync on a separate schedule
func (i *Indexer) runBridgeSyncLoop(ctx context.Context, interval time.Duration) {
	i.logger.Info("starting bridge sync loop", zap.Duration("interval", interval))

	// Run immediately on startup
	i.syncBridgeData(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("bridge sync loop stopped")
			return
		case <-ticker.C:
			i.syncBridgeData(ctx)
		}
	}
}

// syncBridgeData syncs wrap and unwrap token requests from the bridge
func (i *Indexer) syncBridgeData(ctx context.Context) {
	i.logger.Info("bridge sync: starting")

	if err := i.updateBridgeWrapRequests(ctx); err != nil {
		i.logger.Warn("bridge sync: failed to update wrap requests", zap.Error(err))
	}

	if err := i.updateBridgeUnwrapRequests(ctx); err != nil {
		i.logger.Warn("bridge sync: failed to update unwrap requests", zap.Error(err))
	}

	i.logger.Info("bridge sync: complete")
}

// updateBridgeWrapRequests fetches and stores wrap token requests from the bridge.
// API returns newest-first. We page back until we reach the stop height:
// - Stop height = oldest unfinalized TX height (if any unfinalized exist)
// - Stop height = newest known TX height (if all are finalized)
// - Stop height = 0 means fetch all (no wrap requests in DB yet)
func (i *Indexer) updateBridgeWrapRequests(ctx context.Context) error {
	pageSize := uint32(100)
	pageIndex := uint32(0)

	// Get the height we need to sync back to
	stopHeight, err := i.repos.Bridge.GetWrapSyncStopHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get wrap sync stop height: %w", err)
	}

	i.logger.Debug("wrap sync starting", zap.Int64("stopHeight", stopHeight))

	for {
		wrapList, err := i.client.BridgeApi.GetAllWrapTokenRequests(pageIndex, pageSize)
		if err != nil {
			return err
		}

		if len(wrapList.List) == 0 {
			break
		}

		// Track if we've reached or passed the stop height
		reachedStopHeight := false

		for _, w := range wrapList.List {
			wrapRequest := &models.WrapTokenRequest{
				ID:                      w.Id.String(),
				NetworkClass:            int(w.NetworkClass),
				ChainID:                 int(w.ChainId),
				ToAddress:               w.ToAddress,
				TokenStandard:           w.TokenStandard.String(),
				TokenAddress:            w.TokenAddress,
				Amount:                  w.Amount.Int64(),
				Fee:                     w.Fee.Int64(),
				Signature:               w.Signature,
				CreationMomentumHeight:  int64(w.CreationMomentumHeight),
				ConfirmationsToFinality: int(w.ConfirmationsToFinality),
			}
			if err := i.repos.Bridge.UpsertWrapRequest(ctx, wrapRequest); err != nil {
				i.logger.Warn("failed to upsert wrap request", zap.String("id", w.Id.String()), zap.Error(err))
			}

			// Check if we've reached the stop height
			if stopHeight > 0 && int64(w.CreationMomentumHeight) <= stopHeight {
				reachedStopHeight = true
			}
		}

		// Stop if we've reached the stop height
		if reachedStopHeight {
			i.logger.Debug("wrap sync reached stop height",
				zap.Uint32("pageIndex", pageIndex),
				zap.Int64("stopHeight", stopHeight))
			break
		}

		// Check if we've fetched all pages
		if len(wrapList.List) < int(pageSize) {
			break
		}
		pageIndex++
	}

	i.logger.Info("bridge sync: wrap requests done", zap.Uint32("pagesProcessed", pageIndex+1), zap.Int64("stopHeight", stopHeight))
	return nil
}

// updateBridgeUnwrapRequests fetches and stores unwrap token requests from the bridge
// Stops early only when an entire page consists of records that exist AND are finalized in our DB
func (i *Indexer) updateBridgeUnwrapRequests(ctx context.Context) error {
	pageSize := uint32(100)
	pageIndex := uint32(0)

	// Get the stop height - oldest unfinalized TX or newest known height
	stopHeight, err := i.repos.Bridge.GetUnwrapSyncStopHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unwrap sync stop height: %w", err)
	}
	i.logger.Debug("unwrap sync starting", zap.Int64("stopHeight", stopHeight))

	for {
		unwrapList, err := i.client.BridgeApi.GetAllUnwrapTokenRequests(pageIndex, pageSize)
		if err != nil {
			return err
		}

		if len(unwrapList.List) == 0 {
			break
		}

		reachedStopHeight := false

		for _, u := range unwrapList.List {
			unwrapRequest := &models.UnwrapTokenRequest{
				TransactionHash:            u.TransactionHash.String(),
				LogIndex:                   int64(u.LogIndex),
				NetworkClass:               int(u.NetworkClass),
				ChainID:                    int(u.ChainId),
				ToAddress:                  u.ToAddress.String(),
				TokenStandard:              u.TokenStandard.String(),
				TokenAddress:               u.TokenAddress,
				Amount:                     u.Amount.Int64(),
				Signature:                  u.Signature,
				RegistrationMomentumHeight: int64(u.RegistrationMomentumHeight),
				Redeemed:                   u.Redeemed > 0,
				Revoked:                    u.Revoked > 0,
				RedeemableIn:               int64(u.RedeemableIn),
			}
			if err := i.repos.Bridge.UpsertUnwrapRequest(ctx, unwrapRequest); err != nil {
				i.logger.Warn("failed to upsert unwrap request",
					zap.String("txHash", u.TransactionHash.String()),
					zap.Int64("logIndex", int64(u.LogIndex)),
					zap.Error(err))
			}

			// Check if we've reached the stop height
			// API returns newest-first, so we stop when we reach or pass the stop height
			if stopHeight > 0 && int64(u.RegistrationMomentumHeight) <= stopHeight {
				reachedStopHeight = true
			}
		}

		// Stop if we've reached the stop height
		if reachedStopHeight {
			break
		}

		// Check if we've fetched all pages
		if len(unwrapList.List) < int(pageSize) {
			break
		}
		pageIndex++
	}

	i.logger.Info("bridge sync: unwrap requests done", zap.Uint32("pagesProcessed", pageIndex+1), zap.Int64("stopHeight", stopHeight))
	return nil
}

// getPillarOwnerAddress returns the owner address for a pillar name
func (i *Indexer) getPillarOwnerAddress(name string) string {
	i.pillarMu.RLock()
	defer i.pillarMu.RUnlock()
	return i.pillarNameToOwner[name]
}

// getVotingID computes the voting ID for a project or phase by encoding/decoding
// a VoteByName call. This mimics how the protocol computes the voting ID.
func (i *Indexer) getVotingID(id string) string {
	hash, err := types.HexToHash(id)
	if err != nil {
		i.logger.Warn("getVotingID: invalid hash", zap.String("id", id), zap.Error(err))
		return id
	}

	// Encode VoteByName(id, "", 0) using the Accelerator ABI
	encoded, err := embedded.Accelerator.EncodeFunction("VoteByName", []interface{}{hash, "", uint8(0)})
	if err != nil {
		i.logger.Warn("getVotingID: encode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	// Decode to get the voting ID (first parameter)
	decoded, err := embedded.Accelerator.DecodeFunction(encoded)
	if err != nil {
		i.logger.Warn("getVotingID: decode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	if len(decoded) > 0 {
		if h, ok := decoded[0].(types.Hash); ok {
			return h.String()
		}
	}

	return id
}

// getFusionCancelID computes the cancel ID for a fusion by encoding/decoding
// a CancelFuse call. This mimics how the protocol computes the cancel ID.
func (i *Indexer) getFusionCancelID(id string) string {
	hash, err := types.HexToHash(id)
	if err != nil {
		i.logger.Warn("getFusionCancelID: invalid hash", zap.String("id", id), zap.Error(err))
		return id
	}

	// Encode CancelFuse(id) using the Plasma ABI
	encoded, err := embedded.Plasma.EncodeFunction("CancelFuse", []interface{}{hash})
	if err != nil {
		i.logger.Warn("getFusionCancelID: encode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	// Decode to get the cancel ID (first parameter)
	decoded, err := embedded.Plasma.DecodeFunction(encoded)
	if err != nil {
		i.logger.Warn("getFusionCancelID: decode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	if len(decoded) > 0 {
		if h, ok := decoded[0].(types.Hash); ok {
			return h.String()
		}
	}

	return id
}

// getStakeCancelID computes the cancel ID for a stake by encoding/decoding
// a Cancel call. This mimics how the protocol computes the cancel ID.
func (i *Indexer) getStakeCancelID(id string) string {
	hash, err := types.HexToHash(id)
	if err != nil {
		i.logger.Warn("getStakeCancelID: invalid hash", zap.String("id", id), zap.Error(err))
		return id
	}

	// Encode Cancel(id) using the Stake ABI
	encoded, err := embedded.Stake.EncodeFunction("Cancel", []interface{}{hash})
	if err != nil {
		i.logger.Warn("getStakeCancelID: encode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	// Decode to get the cancel ID (first parameter)
	decoded, err := embedded.Stake.DecodeFunction(encoded)
	if err != nil {
		i.logger.Warn("getStakeCancelID: decode failed", zap.String("id", id), zap.Error(err))
		return id
	}

	if len(decoded) > 0 {
		if h, ok := decoded[0].(types.Hash); ok {
			return h.String()
		}
	}

	return id
}

// GetRepositories returns the repository instances
func (i *Indexer) GetRepositories() *repository.Repositories {
	return i.repos
}

// GetPillars returns the cached pillars
func (i *Indexer) GetPillars() []*models.Pillar {
	i.pillarMu.RLock()
	defer i.pillarMu.RUnlock()
	result := make([]*models.Pillar, len(i.pillars))
	copy(result, i.pillars)
	return result
}
