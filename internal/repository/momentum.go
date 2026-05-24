package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type MomentumRepository struct {
	pool *pgxpool.Pool
}

func NewMomentumRepository(pool *pgxpool.Pool) *MomentumRepository {
	return &MomentumRepository{pool: pool}
}

// GetLatestHeight returns the latest indexed momentum height
func (r *MomentumRepository) GetLatestHeight(ctx context.Context) (uint64, error) {
	var height *int64
	err := r.pool.QueryRow(ctx, "SELECT MAX(height) FROM momentums").Scan(&height)
	if err != nil {
		return 0, err
	}
	if height == nil {
		return 0, nil
	}
	return uint64(*height), nil
}

// Insert inserts a momentum, ignoring conflicts
func (r *MomentumRepository) Insert(ctx context.Context, m *models.Momentum) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (height) DO NOTHING`,
		m.Height, m.Hash, m.Timestamp, m.TxCount, m.Producer, m.ProducerOwner, m.ProducerName)
	return err
}

// InsertBatch inserts multiple momentums using a batch
func (r *MomentumRepository) InsertBatch(ctx context.Context, batch *pgx.Batch, m *models.Momentum) {
	batch.Queue(`
		INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (height) DO NOTHING`,
		m.Height, m.Hash, m.Timestamp, m.TxCount, m.Producer, m.ProducerOwner, m.ProducerName)
}

// GetByHeight retrieves a momentum by height
func (r *MomentumRepository) GetByHeight(ctx context.Context, height uint64) (*models.Momentum, error) {
	var m models.Momentum
	err := r.pool.QueryRow(ctx, `
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums WHERE height = $1`, height).Scan(
		&m.Height, &m.Hash, &m.Timestamp, &m.TxCount, &m.Producer, &m.ProducerOwner, &m.ProducerName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetLatest returns the most recent momentum (highest height). Returns
// pgx.ErrNoRows on an empty table.
func (r *MomentumRepository) GetLatest(ctx context.Context) (*models.Momentum, error) {
	var m models.Momentum
	err := r.pool.QueryRow(ctx, `
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums ORDER BY height DESC LIMIT 1`).Scan(
		&m.Height, &m.Hash, &m.Timestamp, &m.TxCount, &m.Producer, &m.ProducerOwner, &m.ProducerName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List returns momentums ordered by height (sort = "asc" or default desc),
// along with the total count for pagination metadata.
//
// Two queries by design. The original pattern inlined COUNT(*) OVER ()
// in the SELECT, which forced Postgres to scan every row of the
// (currently ~13M-row) momentums table even on page 1; production
// requests blew past the API's 30s WriteTimeout. The page query now
// reads only LIMIT rows via the height index, and the total comes
// from MAX(height) — O(log N) against the PK, ~15ms on a 13M-row
// table.
//
// MAX(height) is technically an upper bound: gaps in the height
// sequence (filled by backfill) make it >= the true row count.
// Acceptable for pagination metadata; an exact COUNT would be 1000x
// slower for a meaningless precision gain.
func (r *MomentumRepository) List(ctx context.Context, opts ListOpts) ([]*models.Momentum, int64, error) {
	query := fmt.Sprintf(`
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums
		ORDER BY height %s
		LIMIT $1 OFFSET $2`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.Momentum
	for rows.Next() {
		var m models.Momentum
		if err := rows.Scan(&m.Height, &m.Hash, &m.Timestamp, &m.TxCount,
			&m.Producer, &m.ProducerOwner, &m.ProducerName); err != nil {
			return nil, 0, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var maxHeight *int64
	if err := r.pool.QueryRow(ctx, `SELECT MAX(height) FROM momentums`).Scan(&maxHeight); err != nil {
		return nil, 0, err
	}
	var total int64
	if maxHeight != nil {
		total = *maxHeight
	}
	return out, total, nil
}

// ListByProducer returns momentums whose producer matches the address,
// newest-first. Used to power "blocks produced by pillar" queries.
func (r *MomentumRepository) ListByProducer(ctx context.Context, producer string, opts ListOpts) ([]*models.Momentum, int64, error) {
	if producer == "" {
		return nil, 0, errors.New("producer is required")
	}
	query := fmt.Sprintf(`
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name,
			COUNT(*) OVER () AS total
		FROM momentums
		WHERE producer = $1
		ORDER BY height %s
		LIMIT $2 OFFSET $3`, orderClause(opts.Sort))
	rows, err := r.pool.Query(ctx, query, producer, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out   []*models.Momentum
		total int64
	)
	for rows.Next() {
		var m models.Momentum
		if err := rows.Scan(&m.Height, &m.Hash, &m.Timestamp, &m.TxCount,
			&m.Producer, &m.ProducerOwner, &m.ProducerName, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM momentums WHERE producer = $1`, producer)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// ListByHeightRange returns momentums whose height is in [from, to]
// inclusive, ordered by height ascending. Powers the WS stream's
// replay/catch-up path — a single range scan against the PK index
// instead of `to-from+1` point lookups.
//
// The caller is responsible for sizing the range. Internally we cap
// at the limit argument (0 means no cap, callers should always pass
// a sensible bound — the stream handler uses 10,000).
func (r *MomentumRepository) ListByHeightRange(ctx context.Context, from, to uint64, limit int) ([]*models.Momentum, error) {
	if from > to {
		return nil, nil
	}
	q := `
		SELECT height, hash, timestamp, tx_count, producer, producer_owner, producer_name
		FROM momentums
		WHERE height >= $1 AND height <= $2
		ORDER BY height ASC`
	args := []interface{}{from, to}
	if limit > 0 {
		q += ` LIMIT $3`
		args = append(args, limit)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Momentum
	for rows.Next() {
		var m models.Momentum
		if err := rows.Scan(&m.Height, &m.Hash, &m.Timestamp, &m.TxCount,
			&m.Producer, &m.ProducerOwner, &m.ProducerName); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}
