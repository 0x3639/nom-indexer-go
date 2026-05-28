package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

const syncStatusUpsertSQL = `
INSERT INTO indexer_sync_status (
    id, db_height, znnd_frontier_height, znnd_target_height,
    drift_momentums, node_lag_momentums, state, consecutive_bad_checks,
    active_node_url, active_node_label, chain_identifier,
    failed_over_at, last_progress_at, checked_at
) VALUES (
    1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) ON CONFLICT (id) DO UPDATE SET
    db_height              = EXCLUDED.db_height,
    znnd_frontier_height   = EXCLUDED.znnd_frontier_height,
    znnd_target_height     = EXCLUDED.znnd_target_height,
    drift_momentums        = EXCLUDED.drift_momentums,
    node_lag_momentums     = EXCLUDED.node_lag_momentums,
    state                  = EXCLUDED.state,
    consecutive_bad_checks = EXCLUDED.consecutive_bad_checks,
    active_node_url        = EXCLUDED.active_node_url,
    active_node_label      = EXCLUDED.active_node_label,
    chain_identifier       = EXCLUDED.chain_identifier,
    failed_over_at         = EXCLUDED.failed_over_at,
    last_progress_at       = EXCLUDED.last_progress_at,
    checked_at             = EXCLUDED.checked_at`

const syncStatusGetSQL = `
SELECT db_height, znnd_frontier_height, znnd_target_height,
       drift_momentums, node_lag_momentums, state, consecutive_bad_checks,
       active_node_url, active_node_label, chain_identifier,
       failed_over_at, last_progress_at, checked_at
  FROM indexer_sync_status WHERE id = 1`

// SyncStatusRepository manages the singleton indexer_sync_status row.
type SyncStatusRepository struct {
	pool *pgxpool.Pool
}

// NewSyncStatusRepository constructs a SyncStatusRepository backed by pool.
func NewSyncStatusRepository(pool *pgxpool.Pool) *SyncStatusRepository {
	return &SyncStatusRepository{pool: pool}
}

// Upsert writes (or overwrites) the singleton sync-status row (id=1).
func (r *SyncStatusRepository) Upsert(ctx context.Context, s *models.SyncStatus) error {
	_, err := r.pool.Exec(ctx, syncStatusUpsertSQL,
		s.DBHeight, s.ZnndFrontierHeight, s.ZnndTargetHeight,
		s.DriftMomentums, s.NodeLagMomentums, s.State, s.ConsecutiveBadChecks,
		s.ActiveNodeURL, s.ActiveNodeLabel, s.ChainIdentifier,
		s.FailedOverAt, s.LastProgressAt, s.CheckedAt)
	if err != nil {
		return fmt.Errorf("SyncStatusRepository.Upsert: %w", err)
	}
	return nil
}

// Get retrieves the singleton sync-status row. Returns a wrapped pgx.ErrNoRows
// when the row has never been written so callers can errors.Is it.
func (r *SyncStatusRepository) Get(ctx context.Context) (*models.SyncStatus, error) {
	var s models.SyncStatus
	err := r.pool.QueryRow(ctx, syncStatusGetSQL).Scan(
		&s.DBHeight, &s.ZnndFrontierHeight, &s.ZnndTargetHeight,
		&s.DriftMomentums, &s.NodeLagMomentums, &s.State, &s.ConsecutiveBadChecks,
		&s.ActiveNodeURL, &s.ActiveNodeLabel, &s.ChainIdentifier,
		&s.FailedOverAt, &s.LastProgressAt, &s.CheckedAt)
	if err != nil {
		return nil, fmt.Errorf("SyncStatusRepository.Get: %w", err)
	}
	return &s, nil
}
