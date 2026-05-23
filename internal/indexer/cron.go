package indexer

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// runCronLoop schedules updatePillarVotingActivity and updateTokenHolderCounts
// on independent intervals. Ports the Dart reference's `_runCron` (main.dart).
//
// Both jobs run once on startup and then on the configured interval. They
// share the loop goroutine and back off via ticker.C; ctx cancellation stops
// them cleanly.
func (i *Indexer) runCronLoop(ctx context.Context, votingActivityInterval, tokenHoldersInterval time.Duration) {
	i.logger.Info("starting cron loop",
		zap.Duration("voting_activity_interval", votingActivityInterval),
		zap.Duration("token_holders_interval", tokenHoldersInterval))

	// Run once on startup so dashboards have data immediately.
	i.runVotingActivity(ctx)
	i.runTokenHolderCounts(ctx)

	votingTicker := time.NewTicker(votingActivityInterval)
	defer votingTicker.Stop()
	holderTicker := time.NewTicker(tokenHoldersInterval)
	defer holderTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("cron loop stopped")
			return
		case <-votingTicker.C:
			i.runVotingActivity(ctx)
		case <-holderTicker.C:
			i.runTokenHolderCounts(ctx)
		}
	}
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

