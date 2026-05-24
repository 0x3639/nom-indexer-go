// Package handlers contains the HTTP handler functions backing the
// /api/v1 routes. Handlers are grouped by domain (status, momentums,
// accounts, …) and depend only on the repository layer and the httpx
// helpers — they do not know about chi or the middleware stack so they
// stay testable with net/http/httptest.
package handlers

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
)

// writeRepoError translates common repository errors into RFC 7807
// problems. pgx.ErrNoRows becomes 404; everything else becomes 500 so
// the operator sees it in logs without leaking SQL detail to clients.
func writeRepoError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	httpx.WriteProblem(w, http.StatusInternalServerError, "internal_error", "database error")
}
