package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type PillarUpdateRepository struct {
	pool *pgxpool.Pool
}

func NewPillarUpdateRepository(pool *pgxpool.Pool) *PillarUpdateRepository {
	return &PillarUpdateRepository{pool: pool}
}

// Insert inserts a pillar update
func (r *PillarUpdateRepository) Insert(ctx context.Context, pu *models.PillarUpdate) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pillar_updates (name, owner_address, producer_address, withdraw_address,
			momentum_timestamp, momentum_height, momentum_hash,
			give_momentum_reward_percentage, give_delegate_reward_percentage)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		pu.Name, pu.OwnerAddress, pu.ProducerAddress, pu.WithdrawAddress,
		pu.MomentumTimestamp, pu.MomentumHeight, pu.MomentumHash,
		pu.GiveMomentumRewardPercentage, pu.GiveDelegateRewardPercentage)
	return err
}

// InsertBatch adds a pillar update insert to a batch
func (r *PillarUpdateRepository) InsertBatch(batch *pgx.Batch, pu *models.PillarUpdate) {
	batch.Queue(`
		INSERT INTO pillar_updates (name, owner_address, producer_address, withdraw_address,
			momentum_timestamp, momentum_height, momentum_hash,
			give_momentum_reward_percentage, give_delegate_reward_percentage)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		pu.Name, pu.OwnerAddress, pu.ProducerAddress, pu.WithdrawAddress,
		pu.MomentumTimestamp, pu.MomentumHeight, pu.MomentumHash,
		pu.GiveMomentumRewardPercentage, pu.GiveDelegateRewardPercentage)
}

// GetOwnerAddressAtHeight retrieves the pillar owner at a specific height by withdraw address
func (r *PillarUpdateRepository) GetOwnerAddressAtHeight(ctx context.Context, withdrawAddress string, height int64) (string, error) {
	var ownerAddress string
	err := r.pool.QueryRow(ctx, `
		SELECT owner_address FROM pillar_updates
		WHERE withdraw_address = $1 AND momentum_height <= $2
		ORDER BY id DESC LIMIT 1`,
		withdrawAddress, height).Scan(&ownerAddress)
	if err != nil {
		return "", err
	}
	return ownerAddress, nil
}

// GetInfoAtHeightByProducer retrieves pillar info at a specific height by producer address
func (r *PillarUpdateRepository) GetInfoAtHeightByProducer(ctx context.Context, producerAddress string, height int64) (string, string, error) {
	var ownerAddress, name string
	err := r.pool.QueryRow(ctx, `
		SELECT owner_address, name FROM pillar_updates
		WHERE producer_address = $1 AND momentum_height <= $2
		ORDER BY id DESC LIMIT 1`,
		producerAddress, height).Scan(&ownerAddress, &name)
	if err != nil {
		return "", "", err
	}
	return ownerAddress, name, nil
}
