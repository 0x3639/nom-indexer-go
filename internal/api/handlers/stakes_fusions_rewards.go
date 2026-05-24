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

type stakesRepo interface {
	List(ctx context.Context, activeOnly bool, opts repository.ListOpts) ([]*models.Stake, int64, error)
	ListByAddress(ctx context.Context, address string, activeOnly bool, opts repository.ListOpts) ([]*models.Stake, int64, error)
}

// StakesList handles GET /api/v1/stakes. Active stakes only by default.
func StakesList(repo stakesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		activeOnly := !boolQuery(r, "include_inactive", false)
		rows, total, err := repo.List(r.Context(), activeOnly, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromStakes(rows), p.Page, p.PageSize, total))
	}
}

// StakesByAddress handles GET /api/v1/accounts/{address}/stakes.
func StakesByAddress(repo stakesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		activeOnly := !boolQuery(r, "include_inactive", false)
		rows, total, err := repo.ListByAddress(r.Context(), addr, activeOnly, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromStakes(rows), p.Page, p.PageSize, total))
	}
}

type fusionsRepo interface {
	List(ctx context.Context, activeOnly bool, opts repository.ListOpts) ([]*models.Fusion, int64, error)
	ListByAddress(ctx context.Context, address string, activeOnly bool, opts repository.ListOpts) ([]*models.Fusion, int64, error)
}

// FusionsList handles GET /api/v1/fusions. Active fusions only by default.
func FusionsList(repo fusionsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		activeOnly := !boolQuery(r, "include_inactive", false)
		rows, total, err := repo.List(r.Context(), activeOnly, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromFusions(rows), p.Page, p.PageSize, total))
	}
}

// FusionsByAddress handles GET /api/v1/accounts/{address}/fusions.
// Matches address on either funder OR beneficiary side.
func FusionsByAddress(repo fusionsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		activeOnly := !boolQuery(r, "include_inactive", false)
		rows, total, err := repo.ListByAddress(r.Context(), addr, activeOnly, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromFusions(rows), p.Page, p.PageSize, total))
	}
}

type rewardsRepo interface {
	CumulativeByAddress(ctx context.Context, address string) ([]*models.CumulativeReward, error)
	HistoryByAddress(ctx context.Context, address string, opts repository.ListOpts) ([]*models.RewardTransaction, int64, error)
}

// RewardsCumulative handles GET /api/v1/accounts/{address}/rewards/cumulative.
func RewardsCumulative(repo rewardsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		rows, err := repo.CumulativeByAddress(r.Context(), addr)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data": dto.FromCumulativeRewards(rows),
		})
	}
}

// RewardsHistory handles GET /api/v1/accounts/{address}/rewards.
func RewardsHistory(repo rewardsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "address")
		if addr == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_address", "address is required")
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.HistoryByAddress(r.Context(), addr, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromRewardTransactions(rows), p.Page, p.PageSize, total))
	}
}
