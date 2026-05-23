package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type PillarRepository struct {
	pool *pgxpool.Pool
}

func NewPillarRepository(pool *pgxpool.Pool) *PillarRepository {
	return &PillarRepository{pool: pool}
}

// Upsert inserts or updates a pillar
func (r *PillarRepository) Upsert(ctx context.Context, p *models.Pillar) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pillars (owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (owner_address) DO UPDATE SET
			producer_address = EXCLUDED.producer_address,
			withdraw_address = EXCLUDED.withdraw_address,
			name = EXCLUDED.name,
			rank = EXCLUDED.rank,
			give_momentum_reward_percentage = EXCLUDED.give_momentum_reward_percentage,
			give_delegate_reward_percentage = EXCLUDED.give_delegate_reward_percentage,
			is_revocable = EXCLUDED.is_revocable,
			revoke_cooldown = EXCLUDED.revoke_cooldown,
			revoke_timestamp = EXCLUDED.revoke_timestamp,
			weight = EXCLUDED.weight,
			epoch_produced_momentums = EXCLUDED.epoch_produced_momentums,
			epoch_expected_momentums = EXCLUDED.epoch_expected_momentums`,
		p.OwnerAddress, p.ProducerAddress, p.WithdrawAddress, p.Name, p.Rank,
		p.GiveMomentumRewardPercentage, p.GiveDelegateRewardPercentage, p.IsRevocable,
		p.RevokeCooldown, p.RevokeTimestamp, p.Weight, p.EpochProducedMomentums, p.EpochExpectedMomentums)
	return err
}

// UpdateSpawnInfo updates the spawn timestamp and slot cost
func (r *PillarRepository) UpdateSpawnInfo(ctx context.Context, ownerAddress string, spawnTimestamp int64, slotCostQsr int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pillars SET spawn_timestamp = $2, slot_cost_qsr = $3
		WHERE owner_address = $1`,
		ownerAddress, spawnTimestamp, slotCostQsr)
	return err
}

// UpdateSpawnInfoBatch adds a spawn info update to a batch
func (r *PillarRepository) UpdateSpawnInfoBatch(batch *pgx.Batch, ownerAddress string, spawnTimestamp int64, slotCostQsr int64) {
	batch.Queue(`
		UPDATE pillars SET spawn_timestamp = $2, slot_cost_qsr = $3
		WHERE owner_address = $1`,
		ownerAddress, spawnTimestamp, slotCostQsr)
}

// SetAsRevoked marks an existing pillar as revoked. Historical fields are
// preserved so queries against revoked pillars still have meaningful data;
// only is_revoked and revoke_timestamp are touched.
//
// If the pillar row doesn't exist yet (rare — would mean we observed a Revoke
// before a Register/cached-data sync), insert a minimal row so the revocation
// isn't silently dropped.
func (r *PillarRepository) SetAsRevoked(ctx context.Context, ownerAddress, name string, revokeTimestamp int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pillars (owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked)
		VALUES ($1, '', '', $2, 0, 0, 0, false, 0, $3, 0, 0, 0, 0, 0, 0, 0, true)
		ON CONFLICT (owner_address) DO UPDATE SET
			revoke_timestamp = EXCLUDED.revoke_timestamp,
			is_revoked = true`,
		ownerAddress, name, revokeTimestamp)
	return err
}

// SetAsRevokedBatch adds a revoke update to a batch. See SetAsRevoked.
func (r *PillarRepository) SetAsRevokedBatch(batch *pgx.Batch, ownerAddress, name string, revokeTimestamp int64) {
	batch.Queue(`
		INSERT INTO pillars (owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked)
		VALUES ($1, '', '', $2, 0, 0, 0, false, 0, $3, 0, 0, 0, 0, 0, 0, 0, true)
		ON CONFLICT (owner_address) DO UPDATE SET
			revoke_timestamp = EXCLUDED.revoke_timestamp,
			is_revoked = true`,
		ownerAddress, name, revokeTimestamp)
}

// IncrementMomentumCount increments the produced momentum count
func (r *PillarRepository) IncrementMomentumCount(ctx context.Context, ownerAddress string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pillars SET produced_momentum_count = produced_momentum_count + 1
		WHERE owner_address = $1`,
		ownerAddress)
	return err
}

// IncrementMomentumCountBatch adds a produced-momentum counter update to a batch.
func (r *PillarRepository) IncrementMomentumCountBatch(batch *pgx.Batch, ownerAddress string) {
	batch.Queue(`
		UPDATE pillars SET produced_momentum_count = produced_momentum_count + 1
		WHERE owner_address = $1`,
		ownerAddress)
}

// UpdateVotingActivity updates the voting activity
func (r *PillarRepository) UpdateVotingActivity(ctx context.Context, ownerAddress string, votingActivity float32) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pillars SET voting_activity = $2
		WHERE owner_address = $1`,
		ownerAddress, votingActivity)
	return err
}

// GetByName retrieves a pillar by name
func (r *PillarRepository) GetByName(ctx context.Context, name string) (*models.Pillar, error) {
	var p models.Pillar
	err := r.pool.QueryRow(ctx, `
		SELECT owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked
		FROM pillars WHERE name = $1`, name).Scan(
		&p.OwnerAddress, &p.ProducerAddress, &p.WithdrawAddress, &p.Name, &p.Rank,
		&p.GiveMomentumRewardPercentage, &p.GiveDelegateRewardPercentage, &p.IsRevocable,
		&p.RevokeCooldown, &p.RevokeTimestamp, &p.Weight, &p.EpochProducedMomentums, &p.EpochExpectedMomentums,
		&p.SlotCostQsr, &p.SpawnTimestamp, &p.VotingActivity, &p.ProducedMomentumCount, &p.IsRevoked)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByProducer retrieves a pillar by producer address
func (r *PillarRepository) GetByProducer(ctx context.Context, producerAddress string) (*models.Pillar, error) {
	var p models.Pillar
	err := r.pool.QueryRow(ctx, `
		SELECT owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked
		FROM pillars WHERE producer_address = $1`, producerAddress).Scan(
		&p.OwnerAddress, &p.ProducerAddress, &p.WithdrawAddress, &p.Name, &p.Rank,
		&p.GiveMomentumRewardPercentage, &p.GiveDelegateRewardPercentage, &p.IsRevocable,
		&p.RevokeCooldown, &p.RevokeTimestamp, &p.Weight, &p.EpochProducedMomentums, &p.EpochExpectedMomentums,
		&p.SlotCostQsr, &p.SpawnTimestamp, &p.VotingActivity, &p.ProducedMomentumCount, &p.IsRevoked)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetSpawnTimestamp retrieves spawn timestamp by withdraw address
func (r *PillarRepository) GetSpawnTimestamp(ctx context.Context, withdrawAddress string) (int64, error) {
	var timestamp int64
	err := r.pool.QueryRow(ctx, `
		SELECT spawn_timestamp FROM pillars WHERE withdraw_address = $1`,
		withdrawAddress).Scan(&timestamp)
	if err != nil {
		return -1, err
	}
	return timestamp, nil
}

// GetSpawnTimestampByOwner retrieves spawn timestamp by owner address
func (r *PillarRepository) GetSpawnTimestampByOwner(ctx context.Context, ownerAddress string) (int64, error) {
	var timestamp int64
	err := r.pool.QueryRow(ctx, `
		SELECT spawn_timestamp FROM pillars WHERE owner_address = $1`,
		ownerAddress).Scan(&timestamp)
	if err != nil {
		return -1, err
	}
	return timestamp, nil
}

// IsWithdrawAddress reports whether the given address is the withdraw
// address of any current pillar OR appears as a historical withdraw address
// in pillar_updates. Used by the reward classifier to distinguish a pillar's
// own reward receipt from a delegator's.
func (r *PillarRepository) IsWithdrawAddress(ctx context.Context, address string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pillars WHERE withdraw_address = $1
			UNION
			SELECT 1 FROM pillar_updates WHERE withdraw_address = $1
		)`, address).Scan(&exists)
	return exists, err
}

// GetRevokeTimestamp retrieves revoke timestamp by owner address
func (r *PillarRepository) GetRevokeTimestamp(ctx context.Context, ownerAddress string) (int64, error) {
	var timestamp int64
	err := r.pool.QueryRow(ctx, `
		SELECT revoke_timestamp FROM pillars WHERE owner_address = $1`,
		ownerAddress).Scan(&timestamp)
	if err != nil {
		return 0, err
	}
	return timestamp, nil
}

// List returns pillars ordered by rank ascending. If includeRevoked is
// false (the default for /api/v1/pillars), revoked pillars are excluded.
func (r *PillarRepository) List(ctx context.Context, includeRevoked bool, opts ListOpts) ([]*models.Pillar, int64, error) {
	where := "WHERE is_revoked = false"
	if includeRevoked {
		where = ""
	}
	query := fmt.Sprintf(`
		SELECT owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked,
			COUNT(*) OVER () AS total
		FROM pillars
		%s
		ORDER BY rank ASC
		LIMIT $1 OFFSET $2`, where)
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Pillar
		total int64
	)
	for rows.Next() {
		var p models.Pillar
		if err := rows.Scan(
			&p.OwnerAddress, &p.ProducerAddress, &p.WithdrawAddress, &p.Name, &p.Rank,
			&p.GiveMomentumRewardPercentage, &p.GiveDelegateRewardPercentage, &p.IsRevocable,
			&p.RevokeCooldown, &p.RevokeTimestamp, &p.Weight, &p.EpochProducedMomentums, &p.EpochExpectedMomentums,
			&p.SlotCostQsr, &p.SpawnTimestamp, &p.VotingActivity, &p.ProducedMomentumCount, &p.IsRevoked, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// PillarDelegator is a flattened view of an account currently delegating
// to a pillar. Returned by ListDelegators.
type PillarDelegator struct {
	Address                  string `db:"address"`
	DelegationStartTimestamp int64  `db:"delegation_start_timestamp"`
}

// ListDelegators returns the addresses currently delegated to a pillar,
// paginated. Uses the accounts.delegate column (the indexer keeps this in
// sync with each Delegate / Undelegate event).
func (r *PillarRepository) ListDelegators(ctx context.Context, pillarOwner string, opts ListOpts) ([]*PillarDelegator, int64, error) {
	if pillarOwner == "" {
		return nil, 0, fmt.Errorf("pillar owner is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT address, delegation_start_timestamp, COUNT(*) OVER () AS total
		FROM accounts
		WHERE delegate = $1
		ORDER BY delegation_start_timestamp ASC
		LIMIT $2 OFFSET $3`, pillarOwner, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*PillarDelegator
		total int64
	)
	for rows.Next() {
		var d PillarDelegator
		if err := rows.Scan(&d.Address, &d.DelegationStartTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// GetAll retrieves all pillars
func (r *PillarRepository) GetAll(ctx context.Context) ([]*models.Pillar, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT owner_address, producer_address, withdraw_address, name, rank,
			give_momentum_reward_percentage, give_delegate_reward_percentage, is_revocable,
			revoke_cooldown, revoke_timestamp, weight, epoch_produced_momentums, epoch_expected_momentums,
			slot_cost_qsr, spawn_timestamp, voting_activity, produced_momentum_count, is_revoked
		FROM pillars WHERE is_revoked = false`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pillars []*models.Pillar
	for rows.Next() {
		var p models.Pillar
		err := rows.Scan(
			&p.OwnerAddress, &p.ProducerAddress, &p.WithdrawAddress, &p.Name, &p.Rank,
			&p.GiveMomentumRewardPercentage, &p.GiveDelegateRewardPercentage, &p.IsRevocable,
			&p.RevokeCooldown, &p.RevokeTimestamp, &p.Weight, &p.EpochProducedMomentums, &p.EpochExpectedMomentums,
			&p.SlotCostQsr, &p.SpawnTimestamp, &p.VotingActivity, &p.ProducedMomentumCount, &p.IsRevoked)
		if err != nil {
			return nil, err
		}
		pillars = append(pillars, &p)
	}
	return pillars, nil
}
