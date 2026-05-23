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

type accountBlocksRepo interface {
	List(ctx context.Context, opts repository.ListOpts) ([]*models.AccountBlock, int64, error)
	ListByAddress(ctx context.Context, address string, opts repository.ListOpts) ([]*models.AccountBlock, int64, error)
	GetByHash(ctx context.Context, hash string) (*models.AccountBlock, error)
}

// AccountBlocksList handles GET /api/v1/account_blocks.
func AccountBlocksList(repo accountBlocksRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		sort := httpx.ParseSort(r, "desc")
		rows, total, err := repo.List(r.Context(), repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(), Sort: sort,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromAccountBlocks(rows), p.Page, p.PageSize, total))
	}
}

// AccountBlocksGet handles GET /api/v1/account_blocks/{hash}.
func AccountBlocksGet(repo accountBlocksRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := chi.URLParam(r, "hash")
		if hash == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_hash", "hash is required")
			return
		}
		ab, err := repo.GetByHash(r.Context(), hash)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromAccountBlock(ab))
	}
}

// AccountBlocksByAddress handles GET /api/v1/accounts/{address}/transactions.
// Returns blocks where the address is either sender or recipient.
func AccountBlocksByAddress(repo accountBlocksRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		sort := httpx.ParseSort(r, "desc")
		rows, total, err := repo.ListByAddress(r.Context(), addr, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(), Sort: sort,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromAccountBlocks(rows), p.Page, p.PageSize, total))
	}
}
