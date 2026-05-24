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

type tokensRepo interface {
	List(ctx context.Context, opts repository.ListOpts) ([]*models.Token, int64, error)
	GetByStandard(ctx context.Context, tokenStandard string) (*models.Token, error)
}

type tokenHoldersRepo interface {
	ListByToken(ctx context.Context, tokenStandard string, opts repository.ListOpts) ([]*models.Balance, int64, error)
}

// TokensList handles GET /api/v1/tokens. Sorted by holder_count DESC
// per repository; sort param is intentionally not honored.
func TokensList(repo tokensRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		rows, total, err := repo.List(r.Context(), repository.ListOpts{
			Limit:  p.PageSize,
			Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromTokens(rows), p.Page, p.PageSize, total))
	}
}

// TokensGet handles GET /api/v1/tokens/{token_standard}.
func TokensGet(repo tokensRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		std := chi.URLParam(r, "token_standard")
		if std == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_token_standard", "token_standard is required")
			return
		}
		t, err := repo.GetByStandard(r.Context(), std)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromToken(t))
	}
}

// TokensHolders handles GET /api/v1/tokens/{token_standard}/holders.
// Returns a paginated richlist (balance DESC, excludes zero balances).
func TokensHolders(repo tokenHoldersRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		std := chi.URLParam(r, "token_standard")
		if std == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_token_standard", "token_standard is required")
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListByToken(r.Context(), std, repository.ListOpts{
			Limit:  p.PageSize,
			Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromBalances(rows), p.Page, p.PageSize, total))
	}
}
