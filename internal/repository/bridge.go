package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type BridgeRepository struct {
	pool *pgxpool.Pool
}

func NewBridgeRepository(pool *pgxpool.Pool) *BridgeRepository {
	return &BridgeRepository{pool: pool}
}

// UpsertWrapRequest inserts or updates a wrap token request
func (r *BridgeRepository) UpsertWrapRequest(ctx context.Context, w *models.WrapTokenRequest) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO wrap_token_requests (id, network_class, chain_id, to_address, token_standard,
			token_address, amount, fee, signature, creation_momentum_height, confirmations_to_finality)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			signature = EXCLUDED.signature,
			confirmations_to_finality = EXCLUDED.confirmations_to_finality`,
		w.ID, w.NetworkClass, w.ChainID, w.ToAddress, w.TokenStandard,
		w.TokenAddress, w.Amount, w.Fee, w.Signature, w.CreationMomentumHeight, w.ConfirmationsToFinality)
	return err
}

// UpsertUnwrapRequest inserts or updates an unwrap token request
func (r *BridgeRepository) UpsertUnwrapRequest(ctx context.Context, u *models.UnwrapTokenRequest) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO unwrap_token_requests (transaction_hash, log_index, network_class, chain_id,
			to_address, token_standard, token_address, amount, signature,
			registration_momentum_height, redeemed, revoked, redeemable_in)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (transaction_hash, log_index) DO UPDATE SET
			signature = EXCLUDED.signature,
			redeemed = EXCLUDED.redeemed,
			revoked = EXCLUDED.revoked,
			redeemable_in = EXCLUDED.redeemable_in`,
		u.TransactionHash, u.LogIndex, u.NetworkClass, u.ChainID,
		u.ToAddress, u.TokenStandard, u.TokenAddress, u.Amount, u.Signature,
		u.RegistrationMomentumHeight, u.Redeemed, u.Revoked, u.RedeemableIn)
	return err
}

// GetWrapRequestByID retrieves a wrap request by ID
func (r *BridgeRepository) GetWrapRequestByID(ctx context.Context, id string) (*models.WrapTokenRequest, error) {
	var w models.WrapTokenRequest
	err := r.pool.QueryRow(ctx, `
		SELECT id, network_class, chain_id, to_address, token_standard,
			token_address, amount, fee, signature, creation_momentum_height, confirmations_to_finality
		FROM wrap_token_requests WHERE id = $1`, id).Scan(
		&w.ID, &w.NetworkClass, &w.ChainID, &w.ToAddress, &w.TokenStandard,
		&w.TokenAddress, &w.Amount, &w.Fee, &w.Signature, &w.CreationMomentumHeight, &w.ConfirmationsToFinality)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetUnwrapRequestByTxHash retrieves an unwrap request by transaction hash and log index
func (r *BridgeRepository) GetUnwrapRequestByTxHash(ctx context.Context, txHash string, logIndex int64) (*models.UnwrapTokenRequest, error) {
	var u models.UnwrapTokenRequest
	err := r.pool.QueryRow(ctx, `
		SELECT transaction_hash, log_index, network_class, chain_id,
			to_address, token_standard, token_address, amount, signature,
			registration_momentum_height, redeemed, revoked, redeemable_in
		FROM unwrap_token_requests WHERE transaction_hash = $1 AND log_index = $2`, txHash, logIndex).Scan(
		&u.TransactionHash, &u.LogIndex, &u.NetworkClass, &u.ChainID,
		&u.ToAddress, &u.TokenStandard, &u.TokenAddress, &u.Amount, &u.Signature,
		&u.RegistrationMomentumHeight, &u.Redeemed, &u.Revoked, &u.RedeemableIn)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// IsWrapRequestFinalized checks if a wrap request exists and is finalized (confirmations_to_finality == 0)
// Returns: exists, finalized, error
func (r *BridgeRepository) IsWrapRequestFinalized(ctx context.Context, id string) (bool, bool, error) {
	var confirmations int
	err := r.pool.QueryRow(ctx, `
		SELECT confirmations_to_finality FROM wrap_token_requests WHERE id = $1`, id).Scan(&confirmations)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, confirmations == 0, nil
}

// IsUnwrapRequestFinalized checks if an unwrap request exists and is finalized (redeemed or revoked)
// Returns: exists, finalized, error
func (r *BridgeRepository) IsUnwrapRequestFinalized(ctx context.Context, txHash string, logIndex int64) (bool, bool, error) {
	var redeemed, revoked bool
	err := r.pool.QueryRow(ctx, `
		SELECT redeemed, revoked FROM unwrap_token_requests
		WHERE transaction_hash = $1 AND log_index = $2`, txHash, logIndex).Scan(&redeemed, &revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, redeemed || revoked, nil
}

// GetWrapSyncStopHeight returns the height we should sync back to for wrap requests.
// This is the oldest unfinalized TX height, or if all are finalized, the newest known TX height.
// Returns 0 if no wrap requests exist in DB.
func (r *BridgeRepository) GetWrapSyncStopHeight(ctx context.Context) (int64, error) {
	// First, try to find the oldest unfinalized wrap request
	var height int64
	err := r.pool.QueryRow(ctx, `
		SELECT creation_momentum_height FROM wrap_token_requests
		WHERE confirmations_to_finality > 0
		ORDER BY creation_momentum_height ASC
		LIMIT 1`).Scan(&height)
	if err == nil {
		return height, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	// All are finalized (or none exist), get the newest known TX height
	err = r.pool.QueryRow(ctx, `
		SELECT creation_momentum_height FROM wrap_token_requests
		ORDER BY creation_momentum_height DESC
		LIMIT 1`).Scan(&height)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // No wrap requests in DB
		}
		return 0, err
	}
	return height, nil
}

// GetUnwrapSyncStopHeight returns the height we should sync back to for unwrap requests.
// This is the oldest unfinalized TX height (where both redeemed=false AND revoked=false),
// or if all are finalized, the newest known TX height.
// Unlike wraps, unwrap finalization is user-initiated and can happen out-of-order.
// Returns 0 if no unwrap requests exist in DB.
func (r *BridgeRepository) GetUnwrapSyncStopHeight(ctx context.Context) (int64, error) {
	// First, try to find the oldest unfinalized unwrap request
	// Unfinalized means: redeemed=false AND revoked=false
	var height int64
	err := r.pool.QueryRow(ctx, `
		SELECT registration_momentum_height FROM unwrap_token_requests
		WHERE redeemed = false AND revoked = false
		ORDER BY registration_momentum_height ASC
		LIMIT 1`).Scan(&height)
	if err == nil {
		return height, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	// All are finalized (or none exist), get the newest known TX height
	err = r.pool.QueryRow(ctx, `
		SELECT registration_momentum_height FROM unwrap_token_requests
		ORDER BY registration_momentum_height DESC
		LIMIT 1`).Scan(&height)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // No unwrap requests in DB
		}
		return 0, err
	}
	return height, nil
}
