package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

type bridgeRepo interface {
	ListWraps(ctx context.Context, opts repository.ListOpts) ([]*models.WrapTokenRequest, int64, error)
	ListWrapsByAddress(ctx context.Context, address string, opts repository.ListOpts) ([]*models.WrapTokenRequest, int64, error)
	ListUnwraps(ctx context.Context, opts repository.ListOpts) ([]*models.UnwrapTokenRequest, int64, error)
	ListUnwrapsByAddress(ctx context.Context, address string, opts repository.ListOpts) ([]*models.UnwrapTokenRequest, int64, error)
}

// BridgeWraps handles GET /api/v1/bridge/wraps.
func BridgeWraps(repo bridgeRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListWraps(r.Context(), repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromWrapTokenRequests(rows), p.Page, p.PageSize, total))
	}
}

// BridgeUnwraps handles GET /api/v1/bridge/unwraps.
func BridgeUnwraps(repo bridgeRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListUnwraps(r.Context(), repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromUnwrapTokenRequests(rows), p.Page, p.PageSize, total))
	}
}

// BridgeWrapsByAddress handles GET /api/v1/accounts/{address}/bridge/wraps.
func BridgeWrapsByAddress(repo bridgeRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListWrapsByAddress(r.Context(), addr, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromWrapTokenRequests(rows), p.Page, p.PageSize, total))
	}
}

// BridgeUnwrapsByAddress handles GET /api/v1/accounts/{address}/bridge/unwraps.
func BridgeUnwrapsByAddress(repo bridgeRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListUnwrapsByAddress(r.Context(), addr, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromUnwrapTokenRequests(rows), p.Page, p.PageSize, total))
	}
}
