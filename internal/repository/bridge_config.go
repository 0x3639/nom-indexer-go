package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// BridgeConfigRepository tracks the cached bridge configuration (networks,
// token pairs, admin/guardian set, orchestrator + security info). These rows
// are refreshed on a schedule by syncing against the BridgeApi.
type BridgeConfigRepository struct {
	pool *pgxpool.Pool
}

func NewBridgeConfigRepository(pool *pgxpool.Pool) *BridgeConfigRepository {
	return &BridgeConfigRepository{pool: pool}
}

func (r *BridgeConfigRepository) UpsertNetwork(ctx context.Context, n *models.BridgeNetwork) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_networks (network_class, chain_id, name, contract_address, metadata, last_updated_timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (network_class, chain_id) DO UPDATE SET
			name = EXCLUDED.name,
			contract_address = EXCLUDED.contract_address,
			metadata = EXCLUDED.metadata,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		n.NetworkClass, n.ChainID, n.Name, n.ContractAddress, n.Metadata, n.LastUpdatedTimestamp)
	return err
}

func (r *BridgeConfigRepository) UpsertNetworkToken(ctx context.Context, t *models.BridgeNetworkToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_network_tokens (network_class, chain_id, token_standard, token_address,
			bridgeable, redeemable, owned, min_amount, fee_percentage, redeem_delay, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (network_class, chain_id, token_standard) DO UPDATE SET
			token_address = EXCLUDED.token_address,
			bridgeable = EXCLUDED.bridgeable,
			redeemable = EXCLUDED.redeemable,
			owned = EXCLUDED.owned,
			min_amount = EXCLUDED.min_amount,
			fee_percentage = EXCLUDED.fee_percentage,
			redeem_delay = EXCLUDED.redeem_delay,
			metadata = EXCLUDED.metadata`,
		t.NetworkClass, t.ChainID, t.TokenStandard, t.TokenAddress,
		t.Bridgeable, t.Redeemable, t.Owned, t.MinAmount, t.FeePercentage, t.RedeemDelay, t.Metadata)
	return err
}

func (r *BridgeConfigRepository) UpsertAdmin(ctx context.Context, a *models.BridgeAdmin) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_admin (row_id, administrator, compressed_tss_ecdsa_pubkey,
			decompressed_tss_ecdsa_pubkey, allow_key_gen, halted, unhalted_at,
			unhalt_duration_in_momentums, tss_nonce, metadata, last_updated_timestamp)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (row_id) DO UPDATE SET
			administrator = EXCLUDED.administrator,
			compressed_tss_ecdsa_pubkey = EXCLUDED.compressed_tss_ecdsa_pubkey,
			decompressed_tss_ecdsa_pubkey = EXCLUDED.decompressed_tss_ecdsa_pubkey,
			allow_key_gen = EXCLUDED.allow_key_gen,
			halted = EXCLUDED.halted,
			unhalted_at = EXCLUDED.unhalted_at,
			unhalt_duration_in_momentums = EXCLUDED.unhalt_duration_in_momentums,
			tss_nonce = EXCLUDED.tss_nonce,
			metadata = EXCLUDED.metadata,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		a.Administrator, a.CompressedTssECDSAPubKey, a.DecompressedTssECDSAPubKey,
		a.AllowKeyGen, a.Halted, a.UnhaltedAt, a.UnhaltDurationInMomentums,
		a.TssNonce, a.Metadata, a.LastUpdatedTimestamp)
	return err
}

func (r *BridgeConfigRepository) UpsertGuardian(ctx context.Context, g *models.BridgeGuardian) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_guardians (address, nominated, accepted, last_updated_timestamp)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address) DO UPDATE SET
			nominated = EXCLUDED.nominated,
			accepted = EXCLUDED.accepted,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		g.Address, g.Nominated, g.Accepted, g.LastUpdatedTimestamp)
	return err
}

// MarkGuardiansAbsent flips nominated=false/accepted=false for guardians whose
// last_updated_timestamp is older than `since` and weren't in the latest sync.
// This is how we record that a previously-known guardian has been removed.
func (r *BridgeConfigRepository) MarkGuardiansAbsent(ctx context.Context, since int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bridge_guardians
		SET nominated = false, accepted = false
		WHERE last_updated_timestamp < $1`, since)
	return err
}

func (r *BridgeConfigRepository) UpsertOrchestratorInfo(ctx context.Context, o *models.BridgeOrchestratorInfo) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_orchestrator_info (row_id, window_size, key_gen_threshold,
			confirmations_to_finality, estimated_momentum_time, allow_key_gen_height, last_updated_timestamp)
		VALUES (1, $1, $2, $3, $4, $5, $6)
		ON CONFLICT (row_id) DO UPDATE SET
			window_size = EXCLUDED.window_size,
			key_gen_threshold = EXCLUDED.key_gen_threshold,
			confirmations_to_finality = EXCLUDED.confirmations_to_finality,
			estimated_momentum_time = EXCLUDED.estimated_momentum_time,
			allow_key_gen_height = EXCLUDED.allow_key_gen_height,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		o.WindowSize, o.KeyGenThreshold, o.ConfirmationsToFinality,
		o.EstimatedMomentumTime, o.AllowKeyGenHeight, o.LastUpdatedTimestamp)
	return err
}

func (r *BridgeConfigRepository) UpsertSecurityInfo(ctx context.Context, s *models.BridgeSecurityInfo) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_security_info (row_id, administrator_delay, soft_delay, last_updated_timestamp)
		VALUES (1, $1, $2, $3)
		ON CONFLICT (row_id) DO UPDATE SET
			administrator_delay = EXCLUDED.administrator_delay,
			soft_delay = EXCLUDED.soft_delay,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		s.AdministratorDelay, s.SoftDelay, s.LastUpdatedTimestamp)
	return err
}

func (r *BridgeConfigRepository) GetAdmin(ctx context.Context) (*models.BridgeAdmin, error) {
	var a models.BridgeAdmin
	err := r.pool.QueryRow(ctx, `
		SELECT administrator, compressed_tss_ecdsa_pubkey, decompressed_tss_ecdsa_pubkey,
			allow_key_gen, halted, unhalted_at, unhalt_duration_in_momentums,
			tss_nonce, metadata, last_updated_timestamp
		FROM bridge_admin WHERE row_id = 1`).Scan(
		&a.Administrator, &a.CompressedTssECDSAPubKey, &a.DecompressedTssECDSAPubKey,
		&a.AllowKeyGen, &a.Halted, &a.UnhaltedAt, &a.UnhaltDurationInMomentums,
		&a.TssNonce, &a.Metadata, &a.LastUpdatedTimestamp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}
