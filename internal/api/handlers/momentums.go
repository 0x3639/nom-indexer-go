package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// momentumsRepo is the slice of MomentumRepository the handlers depend
// on. Defined as an interface so handlers tests can swap in a fake.
type momentumsRepo interface {
	GetLatest(ctx context.Context) (*models.Momentum, error)
	GetByHeight(ctx context.Context, height uint64) (*models.Momentum, error)
	List(ctx context.Context, opts repository.ListOpts) ([]*models.Momentum, int64, error)
}

// MomentumsList handles GET /api/v1/momentums.
//
// Query parameters:
//   - page, page_size (offset pagination; see httpx.ParsePagination)
//   - sort: "asc" | "desc" (default "desc")
func MomentumsList(repo momentumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		sort := httpx.ParseSort(r, "desc")

		rows, total, err := repo.List(r.Context(), repository.ListOpts{
			Limit:  p.PageSize,
			Offset: p.Offset(),
			Sort:   sort,
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromMomentums(rows), p.Page, p.PageSize, total))
	}
}

// MomentumsLatest handles GET /api/v1/momentums/latest.
// Returns the highest-height momentum or 404 if the table is empty.
func MomentumsLatest(repo momentumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := repo.GetLatest(r.Context())
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromMomentum(m))
	}
}

// MomentumsGetByHeight handles GET /api/v1/momentums/{height}.
// Rejects non-numeric / negative heights with 400 before touching the DB.
func MomentumsGetByHeight(repo momentumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := chi.URLParam(r, "height")
		height, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_height",
				"height must be a non-negative integer")
			return
		}
		m, err := repo.GetByHeight(r.Context(), height)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromMomentum(m))
	}
}
