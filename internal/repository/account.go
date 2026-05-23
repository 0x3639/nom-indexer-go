package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// Upsert inserts or updates an account
func (r *AccountRepository) Upsert(ctx context.Context, a *models.Account) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO accounts (address, block_count, public_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (address) DO UPDATE SET
			block_count = $2,
			public_key = COALESCE(NULLIF($3, ''), accounts.public_key)`,
		a.Address, a.BlockCount, a.PublicKey)
	return err
}

// UpsertBatch adds an account upsert to a batch
func (r *AccountRepository) UpsertBatch(batch *pgx.Batch, a *models.Account) {
	batch.Queue(`
		INSERT INTO accounts (address, block_count, public_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (address) DO UPDATE SET
			block_count = $2,
			public_key = COALESCE(NULLIF($3, ''), accounts.public_key)`,
		a.Address, a.BlockCount, a.PublicKey)
}

// AddSendBatch increments the address's znn_sent or qsr_sent by the given
// amount (depending on tokenStandard) and bumps first/last activity. Non-ZNN
// /non-QSR tokens update activity only.
func (r *AccountRepository) AddSendBatch(batch *pgx.Batch, address, tokenStandard string, amount, timestamp int64) {
	col := flowColumn(tokenStandard, "sent")
	if col == "" {
		batch.Queue(`
			INSERT INTO accounts (address, block_count, public_key, first_active_at, last_active_at)
			VALUES ($1, 0, '', $2, $2)
			ON CONFLICT (address) DO UPDATE SET
				first_active_at = LEAST(COALESCE(accounts.first_active_at, EXCLUDED.first_active_at), EXCLUDED.first_active_at),
				last_active_at = GREATEST(COALESCE(accounts.last_active_at, EXCLUDED.last_active_at), EXCLUDED.last_active_at)`,
			address, timestamp)
		return
	}
	// String concat is safe here: col comes from flowColumn's whitelist, never user input.
	batch.Queue(`
		INSERT INTO accounts (address, block_count, public_key, `+col+`, first_active_at, last_active_at)
		VALUES ($1, 0, '', $2, $3, $3)
		ON CONFLICT (address) DO UPDATE SET
			`+col+` = accounts.`+col+` + EXCLUDED.`+col+`,
			first_active_at = LEAST(COALESCE(accounts.first_active_at, EXCLUDED.first_active_at), EXCLUDED.first_active_at),
			last_active_at = GREATEST(COALESCE(accounts.last_active_at, EXCLUDED.last_active_at), EXCLUDED.last_active_at)`,
		address, amount, timestamp)
}

// AddReceiveBatch is the receive-side analogue of AddSendBatch.
func (r *AccountRepository) AddReceiveBatch(batch *pgx.Batch, address, tokenStandard string, amount, timestamp int64) {
	col := flowColumn(tokenStandard, "received")
	if col == "" {
		batch.Queue(`
			INSERT INTO accounts (address, block_count, public_key, first_active_at, last_active_at)
			VALUES ($1, 0, '', $2, $2)
			ON CONFLICT (address) DO UPDATE SET
				first_active_at = LEAST(COALESCE(accounts.first_active_at, EXCLUDED.first_active_at), EXCLUDED.first_active_at),
				last_active_at = GREATEST(COALESCE(accounts.last_active_at, EXCLUDED.last_active_at), EXCLUDED.last_active_at)`,
			address, timestamp)
		return
	}
	batch.Queue(`
		INSERT INTO accounts (address, block_count, public_key, `+col+`, first_active_at, last_active_at)
		VALUES ($1, 0, '', $2, $3, $3)
		ON CONFLICT (address) DO UPDATE SET
			`+col+` = accounts.`+col+` + EXCLUDED.`+col+`,
			first_active_at = LEAST(COALESCE(accounts.first_active_at, EXCLUDED.first_active_at), EXCLUDED.first_active_at),
			last_active_at = GREATEST(COALESCE(accounts.last_active_at, EXCLUDED.last_active_at), EXCLUDED.last_active_at)`,
		address, amount, timestamp)
}

// flowColumn maps (token_standard, direction) to the accounts column name.
// Returns "" for non-ZNN/QSR tokens (we don't track per-token flow totals).
func flowColumn(tokenStandard, direction string) string {
	switch tokenStandard {
	case models.ZnnTokenStandard:
		if direction == "sent" {
			return "znn_sent"
		}
		return "znn_received"
	case models.QsrTokenStandard:
		if direction == "sent" {
			return "qsr_sent"
		}
		return "qsr_received"
	default:
		return ""
	}
}

// UpdateDelegate updates the delegate for an account
func (r *AccountRepository) UpdateDelegate(ctx context.Context, address, delegate string, timestamp int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE accounts
		SET delegate = $2, delegation_start_timestamp = $3
		WHERE address = $1`,
		address, delegate, timestamp)
	return err
}

// UpdateDelegateBatch adds a delegate update to a batch
func (r *AccountRepository) UpdateDelegateBatch(batch *pgx.Batch, address, delegate string, timestamp int64) {
	batch.Queue(`
		UPDATE accounts
		SET delegate = $2, delegation_start_timestamp = $3
		WHERE address = $1`,
		address, delegate, timestamp)
}

// GetByAddress retrieves an account by address
func (r *AccountRepository) GetByAddress(ctx context.Context, address string) (*models.Account, error) {
	var a models.Account
	err := r.pool.QueryRow(ctx, `
		SELECT address, block_count, public_key, delegate, delegation_start_timestamp,
			genesis_znn_balance, genesis_qsr_balance,
			znn_sent, znn_received, qsr_sent, qsr_received,
			first_active_at, last_active_at
		FROM accounts WHERE address = $1`, address).Scan(
		&a.Address, &a.BlockCount, &a.PublicKey, &a.Delegate, &a.DelegationStartTimestamp,
		&a.GenesisZnnBalance, &a.GenesisQsrBalance,
		&a.ZnnSent, &a.ZnnReceived, &a.QsrSent, &a.QsrReceived,
		&a.FirstActiveAt, &a.LastActiveAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SetGenesisBalanceBatch records the genesis balance for a (address, token)
// pair. Should be called only during the genesis momentum (height 1).
func (r *AccountRepository) SetGenesisBalanceBatch(batch *pgx.Batch, address, tokenStandard string, balance int64) {
	switch tokenStandard {
	case models.ZnnTokenStandard:
		batch.Queue(`
			INSERT INTO accounts (address, block_count, public_key, genesis_znn_balance)
			VALUES ($1, 0, '', $2)
			ON CONFLICT (address) DO UPDATE SET genesis_znn_balance = EXCLUDED.genesis_znn_balance`,
			address, balance)
	case models.QsrTokenStandard:
		batch.Queue(`
			INSERT INTO accounts (address, block_count, public_key, genesis_qsr_balance)
			VALUES ($1, 0, '', $2)
			ON CONFLICT (address) DO UPDATE SET genesis_qsr_balance = EXCLUDED.genesis_qsr_balance`,
			address, balance)
	}
}
