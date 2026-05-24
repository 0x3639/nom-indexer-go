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

type pillarsRepo interface {
	List(ctx context.Context, includeRevoked bool, opts repository.ListOpts) ([]*models.Pillar, int64, error)
	GetByName(ctx context.Context, name string) (*models.Pillar, error)
	ListDelegators(ctx context.Context, owner string, opts repository.ListOpts) ([]*repository.PillarDelegator, int64, error)
}

// boolQuery reads a true/false query param. Missing or unrecognized → def.
func boolQuery(r *http.Request, name string, def bool) bool {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// PillarsList handles GET /api/v1/pillars. Excludes revoked pillars by
// default; pass ?include_revoked=true to include them.
func PillarsList(repo pillarsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		includeRevoked := boolQuery(r, "include_revoked", false)
		rows, total, err := repo.List(r.Context(), includeRevoked, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromPillars(rows), p.Page, p.PageSize, total))
	}
}

// PillarsGetByName handles GET /api/v1/pillars/{name}.
func PillarsGetByName(repo pillarsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if name == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_name", "name is required")
			return
		}
		p, err := repo.GetByName(r.Context(), name)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromPillar(p))
	}
}

// PillarsDelegators handles GET /api/v1/pillars/{name}/delegators.
// Looks up the pillar's owner address from its name, then queries
// accounts.delegate. 404 if the pillar name is unknown.
func PillarsDelegators(repo pillarsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if name == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_name", "name is required")
			return
		}
		pillar, err := repo.GetByName(r.Context(), name)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListDelegators(r.Context(), pillar.OwnerAddress, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromPillarDelegators(rows), p.Page, p.PageSize, total))
	}
}

// PillarsVotingHistory handles GET /api/v1/pillars/{name}/voting-report.
// Returns one pillar's complete voting record across every project +
// phase, with project + phase names joined server-side and vote codes
// already translated to strings.
func PillarsVotingHistory(repo votingReportRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		if name == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_name", "name is required")
			return
		}
		row, err := repo.PillarVotingHistory(r.Context(), name)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		raws := make([]dto.RawPillarVote, 0, len(row.Votes))
		for _, v := range row.Votes {
			raws = append(raws, dto.RawPillarVote{
				VotingID:          v.VotingID,
				Vote:              v.Vote,
				MomentumHeight:    v.MomentumHeight,
				MomentumTimestamp: v.MomentumTimestamp,
				ProjectID:         v.ProjectID,
				PhaseID:           v.PhaseID,
				ProjectName:       v.ProjectName,
				PhaseName:         v.PhaseName,
			})
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.FromPillarVotingHistory(row.PillarName, row.PillarOwner, raws))
	}
}

type sentinelsRepo interface {
	List(ctx context.Context, activeOnly bool, opts repository.ListOpts) ([]*models.Sentinel, int64, error)
}

// SentinelsList handles GET /api/v1/sentinels. Active only by default;
// pass ?include_inactive=true to show all.
func SentinelsList(repo sentinelsRepo) http.HandlerFunc {
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
			dto.NewPage(dto.FromSentinels(rows), p.Page, p.PageSize, total))
	}
}
