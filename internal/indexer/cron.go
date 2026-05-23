package indexer

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// runCronLoop schedules updatePillarVotingActivity and updateTokenHolderCounts
// on independent intervals. Ports the Dart reference's `_runCron` (main.dart).
//
// Daily stat snapshot jobs (network/token/pillar/bridge) fire on a 1-hour
// ticker so the current day's row is kept up to date even mid-day.
func (i *Indexer) runCronLoop(ctx context.Context, votingActivityInterval, tokenHoldersInterval time.Duration) {
	statsInterval := time.Hour

	i.logger.Info("starting cron loop",
		zap.Duration("voting_activity_interval", votingActivityInterval),
		zap.Duration("token_holders_interval", tokenHoldersInterval),
		zap.Duration("stat_snapshot_interval", statsInterval))

	// Run once on startup so dashboards have data immediately.
	i.runVotingActivity(ctx)
	i.runTokenHolderCounts(ctx)
	i.runStatSnapshots(ctx)

	votingTicker := time.NewTicker(votingActivityInterval)
	defer votingTicker.Stop()
	holderTicker := time.NewTicker(tokenHoldersInterval)
	defer holderTicker.Stop()
	statsTicker := time.NewTicker(statsInterval)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("cron loop stopped")
			return
		case <-votingTicker.C:
			i.runVotingActivity(ctx)
		case <-holderTicker.C:
			i.runTokenHolderCounts(ctx)
		case <-statsTicker.C:
			i.runStatSnapshots(ctx)
		}
	}
}

// runStatSnapshots refreshes the current day's row in each *_stat_histories
// table. Each snapshot job is independent; one failure does not block the
// others.
func (i *Indexer) runStatSnapshots(ctx context.Context) {
	start := time.Now()
	today := time.Now().UTC().Format("2006-01-02")
	// Today as midnight Unix seconds; used to bucket by momentum_timestamp.
	todayStart, err := time.Parse("2006-01-02", today)
	if err != nil {
		i.logger.Warn("stat snapshots: parse today failed", zap.Error(err))
		return
	}
	startTs := todayStart.Unix()
	endTs := startTs + 86400

	if err := i.snapshotNetworkStats(ctx, today, startTs, endTs); err != nil {
		i.logger.Warn("stat snapshots: network failed", zap.Error(err))
	}
	if err := i.snapshotTokenStats(ctx, today); err != nil {
		i.logger.Warn("stat snapshots: tokens failed", zap.Error(err))
	}
	if err := i.snapshotPillarStats(ctx, today); err != nil {
		i.logger.Warn("stat snapshots: pillars failed", zap.Error(err))
	}
	if err := i.snapshotBridgeStats(ctx, today, startTs, endTs); err != nil {
		i.logger.Warn("stat snapshots: bridge failed", zap.Error(err))
	}

	i.logger.Info("stat snapshots refreshed",
		zap.String("date", today),
		zap.Duration("duration", time.Since(start)))
}

func (i *Indexer) snapshotNetworkStats(ctx context.Context, date string, startTs, endTs int64) error {
	stat := &models.NetworkStatHistory{Date: date}

	if err := i.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT SUM(tx_count) FROM momentums), 0)::bigint AS total_tx,
			COALESCE((SELECT SUM(tx_count) FROM momentums
				WHERE timestamp >= $1 AND timestamp < $2), 0)::bigint AS daily_tx,
			COALESCE((SELECT COUNT(*) FROM accounts), 0)::bigint AS total_addresses,
			COALESCE((SELECT COUNT(*) FROM accounts
				WHERE first_active_at >= $1 AND first_active_at < $2), 0)::bigint AS daily_addresses,
			COALESCE((SELECT COUNT(*) FROM accounts
				WHERE last_active_at >= $1 AND last_active_at < $2), 0)::bigint AS active_addresses,
			COALESCE((SELECT COUNT(*) FROM tokens), 0)::bigint AS total_tokens,
			COALESCE((SELECT COUNT(*) FROM stakes), 0)::bigint AS total_stakes,
			COALESCE((SELECT COUNT(*) FROM stakes
				WHERE start_timestamp >= $1 AND start_timestamp < $2), 0)::bigint AS daily_stakes,
			COALESCE((SELECT COUNT(*) FROM fusions), 0)::bigint AS total_fusions,
			COALESCE((SELECT COUNT(*) FROM fusions
				WHERE momentum_timestamp >= $1 AND momentum_timestamp < $2), 0)::bigint AS daily_fusions,
			COALESCE((SELECT COUNT(*) FROM pillars WHERE is_revoked = false), 0)::bigint AS total_pillars,
			COALESCE((SELECT COUNT(*) FROM sentinels WHERE active = true), 0)::bigint AS total_sentinels`,
		startTs, endTs).Scan(
		&stat.TotalTx, &stat.DailyTx, &stat.TotalAddresses, &stat.DailyAddresses,
		&stat.ActiveAddresses, &stat.TotalTokens, &stat.TotalStakes, &stat.DailyStakes,
		&stat.TotalFusions, &stat.DailyFusions, &stat.TotalPillars, &stat.TotalSentinels); err != nil {
		return fmt.Errorf("aggregate network stats: %w", err)
	}

	return i.repos.StatHistory.UpsertNetworkStat(ctx, stat)
}

func (i *Indexer) snapshotTokenStats(ctx context.Context, date string) error {
	tokens, err := i.repos.Token.GetAll(ctx)
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		mints, burns, sumErr := i.repos.TokenEvent.SumDailyMintsBurns(ctx, t.TokenStandard, date)
		if sumErr != nil {
			i.logger.Warn("token stat: sum daily mints/burns failed",
				zap.String("token", t.TokenStandard), zap.Error(sumErr))
			continue
		}
		if err := i.repos.StatHistory.UpsertTokenStat(ctx, &models.TokenStatHistory{
			Date:              date,
			TokenStandard:     t.TokenStandard,
			DailyMinted:       mints,
			DailyBurned:       burns,
			TotalSupply:       t.TotalSupply,
			TotalHolders:      t.HolderCount,
			TotalTransactions: t.TransactionCount,
		}); err != nil {
			i.logger.Warn("token stat upsert failed",
				zap.String("token", t.TokenStandard), zap.Error(err))
		}
	}
	return nil
}

func (i *Indexer) snapshotPillarStats(ctx context.Context, date string) error {
	pillars := i.GetPillars()
	for _, p := range pillars {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delegators, err := i.repos.Delegation.CountActiveByPillar(ctx, p.OwnerAddress)
		if err != nil {
			i.logger.Warn("pillar stat: delegator count failed",
				zap.String("owner", p.OwnerAddress), zap.Error(err))
			delegators = 0
		}
		if err := i.repos.StatHistory.UpsertPillarStat(ctx, &models.PillarStatHistory{
			Date:               date,
			PillarOwnerAddress: p.OwnerAddress,
			Rank:               p.Rank,
			Weight:             p.Weight,
			// Reward totals are left at zero for now; a future job can backfill
			// them from reward_transactions joined against delegations history.
			MomentumRewards: 0,
			DelegateRewards: 0,
			TotalDelegators: delegators,
		}); err != nil {
			i.logger.Warn("pillar stat upsert failed",
				zap.String("owner", p.OwnerAddress), zap.Error(err))
		}
	}
	return nil
}

func (i *Indexer) snapshotBridgeStats(ctx context.Context, date string, startTs, endTs int64) error {
	// Aggregate wrap requests by (network_class, chain_id, token_standard) for today.
	rows, err := i.pool.Query(ctx, `
		SELECT network_class, chain_id, token_standard,
			COUNT(*)::bigint AS wrap_tx,
			COALESCE(SUM(amount), 0)::bigint AS wrapped_amount
		FROM wrap_token_requests
		WHERE creation_momentum_height IN (
			SELECT height FROM momentums WHERE timestamp >= $1 AND timestamp < $2
		)
		GROUP BY network_class, chain_id, token_standard`,
		startTs, endTs)
	if err != nil {
		return fmt.Errorf("query wrap aggregates: %w", err)
	}
	stats := map[string]*models.BridgeStatHistory{}
	for rows.Next() {
		var s models.BridgeStatHistory
		if err := rows.Scan(&s.NetworkClass, &s.ChainID, &s.TokenStandard,
			&s.WrapTxCount, &s.WrappedAmount); err != nil {
			rows.Close()
			return err
		}
		s.Date = date
		s.TotalVolume = s.WrappedAmount
		key := fmt.Sprintf("%d:%d:%s", s.NetworkClass, s.ChainID, s.TokenStandard)
		stats[key] = &s
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Unwrap side.
	rows, err = i.pool.Query(ctx, `
		SELECT network_class, chain_id, token_standard,
			COUNT(*)::bigint AS unwrap_tx,
			COALESCE(SUM(amount), 0)::bigint AS unwrapped_amount
		FROM unwrap_token_requests
		WHERE registration_momentum_height IN (
			SELECT height FROM momentums WHERE timestamp >= $1 AND timestamp < $2
		)
		GROUP BY network_class, chain_id, token_standard`,
		startTs, endTs)
	if err != nil {
		return fmt.Errorf("query unwrap aggregates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var nc, ci int
		var ts string
		var tx, amount int64
		if err := rows.Scan(&nc, &ci, &ts, &tx, &amount); err != nil {
			return err
		}
		key := fmt.Sprintf("%d:%d:%s", nc, ci, ts)
		s := stats[key]
		if s == nil {
			s = &models.BridgeStatHistory{
				Date: date, NetworkClass: nc, ChainID: ci, TokenStandard: ts,
			}
			stats[key] = s
		}
		s.UnwrapTxCount = tx
		s.UnwrappedAmount = amount
		s.TotalVolume += amount
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range stats {
		if err := i.repos.StatHistory.UpsertBridgeStat(ctx, s); err != nil {
			i.logger.Warn("bridge stat upsert failed", zap.Error(err))
		}
	}
	return nil
}

// runVotingActivity recomputes voting_activity for every pillar. The score is
// (distinct proposals voted on) / (proposals eligible to vote on since spawn).
// Mirrors NomIndexer.updatePillarVotingActivity in the Dart reference.
func (i *Indexer) runVotingActivity(ctx context.Context) {
	start := time.Now()
	pillars := i.GetPillars()
	if len(pillars) == 0 {
		i.logger.Debug("voting activity: no pillars cached, skipping")
		return
	}

	updated := 0
	for _, p := range pillars {
		if ctx.Err() != nil {
			return
		}

		spawnTimestamp, err := i.repos.Pillar.GetSpawnTimestampByOwner(ctx, p.OwnerAddress)
		if err != nil {
			i.logger.Debug("voting activity: skip pillar (no spawn ts)",
				zap.String("owner", p.OwnerAddress),
				zap.Error(err))
			continue
		}

		projectIDs, err := i.repos.Project.GetIDsCreatedAtOrAfter(ctx, spawnTimestamp)
		if err != nil {
			i.logger.Warn("voting activity: project lookup failed", zap.Error(err))
			continue
		}
		phaseIDs, err := i.repos.ProjectPhase.GetIDsCreatedAtOrAfter(ctx, spawnTimestamp)
		if err != nil {
			i.logger.Warn("voting activity: phase lookup failed", zap.Error(err))
			continue
		}

		votes := 0
		if len(projectIDs) > 0 {
			n, err := i.repos.Vote.GetVoteCountForProjects(ctx, p.OwnerAddress, projectIDs)
			if err != nil {
				i.logger.Warn("voting activity: project votes failed", zap.Error(err))
				continue
			}
			votes += n
		}
		if len(phaseIDs) > 0 {
			n, err := i.repos.Vote.GetVoteCountForPhases(ctx, p.OwnerAddress, phaseIDs)
			if err != nil {
				i.logger.Warn("voting activity: phase votes failed", zap.Error(err))
				continue
			}
			votes += n
		}

		votable := len(projectIDs) + len(phaseIDs)
		var activity float32
		if votable > 0 {
			activity = float32(votes) / float32(votable)
		}

		if err := i.repos.Pillar.UpdateVotingActivity(ctx, p.OwnerAddress, activity); err != nil {
			i.logger.Warn("voting activity: update failed",
				zap.String("owner", p.OwnerAddress),
				zap.Error(err))
			continue
		}
		updated++
	}

	i.logger.Info("voting activity refreshed",
		zap.Int("pillars", updated),
		zap.Duration("duration", time.Since(start)))
}

// runTokenHolderCounts refreshes holder_count for every token by counting
// addresses with balance > 0. Mirrors NomIndexer.updateTokenHolderCounts.
func (i *Indexer) runTokenHolderCounts(ctx context.Context) {
	start := time.Now()
	tokens, err := i.repos.Token.GetAll(ctx)
	if err != nil {
		i.logger.Warn("token holders: failed to list tokens", zap.Error(err))
		return
	}

	updated := 0
	for _, t := range tokens {
		if ctx.Err() != nil {
			return
		}
		count, err := i.repos.Balance.GetHolderCount(ctx, t.TokenStandard)
		if err != nil {
			i.logger.Warn("token holders: count failed",
				zap.String("token", t.TokenStandard),
				zap.Error(err))
			continue
		}
		if err := i.repos.Token.UpdateHolderCount(ctx, t.TokenStandard, count); err != nil {
			i.logger.Warn("token holders: update failed",
				zap.String("token", t.TokenStandard),
				zap.Error(err))
			continue
		}
		updated++
	}

	i.logger.Info("token holder counts refreshed",
		zap.Int("tokens", updated),
		zap.Duration("duration", time.Since(start)))
}

// ParseCronInterval parses a config interval string ("10m", "1h"), returning
// a typed duration. Defaults to defaultInterval if empty; returns an error if
// the input is non-empty but unparseable.
func ParseCronInterval(s string, defaultInterval time.Duration) (time.Duration, error) {
	if s == "" {
		return defaultInterval, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("parse interval %q: %w", s, err)
	}
	return d, nil
}

