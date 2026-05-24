package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

type accountsRepo interface {
	GetByAddress(ctx context.Context, address string) (*models.Account, error)
}

type accountBalancesRepo interface {
	ListByAddress(ctx context.Context, address string) ([]*models.Balance, error)
}

// AccountsGet handles GET /api/v1/accounts/{address}.
func AccountsGet(repo accountsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		a, err := repo.GetByAddress(r.Context(), addr)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromAccount(a))
	}
}

// AccountsBalances handles GET /api/v1/accounts/{address}/balances.
// Returns every (token, amount) row for the address. No pagination —
// accounts typically hold a handful of tokens.
func AccountsBalances(repo accountBalancesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		rows, err := repo.ListByAddress(r.Context(), addr)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data": dto.FromBalances(rows),
		})
	}
}
