package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DelegationRepository owns the time-bucketed delegations history (one row per
// delegation interval, ended_at nullable for the current/active one).
type DelegationRepository struct {
	pool *pgxpool.Pool
}

func NewDelegationRepository(pool *pgxpool.Pool) *DelegationRepository {
	return &DelegationRepository{pool: pool}
}

// CloseActive ends the delegator's current open delegation, if any, at the
// given timestamp. Returns the pillar that was closed (empty string if none).
// Use the batched variant from inside processMomentum.
func (r *DelegationRepository) CloseActive(ctx context.Context, delegator string, endedAt int64) (string, error) {
	var pillarOwner string
	err := r.pool.QueryRow(ctx, `
		UPDATE delegations SET ended_at = $2
		WHERE delegator_address = $1 AND ended_at IS NULL
		RETURNING pillar_owner_address`,
		delegator, endedAt).Scan(&pillarOwner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return pillarOwner, nil
}

// CloseActiveBatch enqueues a CloseActive update as part of a batched txn.
// Does not return the closed pillar's owner address — see the non-batch
// variant if you need that.
func (r *DelegationRepository) CloseActiveBatch(batch *pgx.Batch, delegator string, endedAt int64) {
	batch.Queue(`
		UPDATE delegations SET ended_at = $2
		WHERE delegator_address = $1 AND ended_at IS NULL`,
		delegator, endedAt)
}

// OpenBatch starts a new active delegation interval. Should be invoked after
// CloseActiveBatch in the same batch when a delegator switches pillars.
func (r *DelegationRepository) OpenBatch(batch *pgx.Batch, delegator, pillarOwner string, startedAt int64) {
	batch.Queue(`
		INSERT INTO delegations (delegator_address, pillar_owner_address, started_at)
		VALUES ($1, $2, $3)`,
		delegator, pillarOwner, startedAt)
}

// CountActiveByPillar returns the number of currently-active delegations to a
// given pillar.
func (r *DelegationRepository) CountActiveByPillar(ctx context.Context, pillarOwner string) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM delegations
		WHERE pillar_owner_address = $1 AND ended_at IS NULL`,
		pillarOwner).Scan(&n)
	return n, err
}

// GetActivePillarFor returns the pillar a delegator is currently delegated to,
// or empty string if none.
func (r *DelegationRepository) GetActivePillarFor(ctx context.Context, delegator string) (string, error) {
	var pillar string
	err := r.pool.QueryRow(ctx, `
		SELECT pillar_owner_address FROM delegations
		WHERE delegator_address = $1 AND ended_at IS NULL LIMIT 1`,
		delegator).Scan(&pillar)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return pillar, nil
}
