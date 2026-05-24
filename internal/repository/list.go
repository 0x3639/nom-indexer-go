package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ListOpts is the canonical pagination + sort shape that every List* method
// in this package accepts. Handlers translate HTTP query parameters into it
// via internal/api/httpx.ParsePagination / ParseSort.
//
// Per-method documentation on each repository explains which column the
// Sort field reorders and what the default direction is.
type ListOpts struct {
	Limit  int    // SQL LIMIT (page_size)
	Offset int    // SQL OFFSET ((page-1)*page_size)
	Sort   string // "asc" / "desc" — anything else is treated as the method's default
}

// orderClause returns "ASC" or "DESC". Any value other than "asc" is
// treated as descending so callers don't have to validate the field.
func orderClause(sort string) string {
	if sort == "asc" {
		return "ASC"
	}
	return "DESC"
}

// fallbackCount runs a separate COUNT(*) query when the paginated
// SELECT returned no rows past the first page. The List* methods compute
// the total via COUNT(*) OVER () inline with the page, which is fast
// and accurate — except when OFFSET skips past every matching row: with
// zero rows in the result set, no row carries the window-function total
// and the caller sees total=0 even when the underlying set is non-empty.
// This helper closes that gap. It is intentionally called only on the
// (rows==0 && offset>0) corner so the hot path keeps the single
// round-trip property.
//
// countQuery must be the SELECT that returns one int64 row, e.g.
//
//	SELECT COUNT(*) FROM momentums WHERE producer = $1
func fallbackCount(ctx context.Context, pool *pgxpool.Pool, countQuery string, args ...interface{}) (int64, error) {
	var total int64
	if err := pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
