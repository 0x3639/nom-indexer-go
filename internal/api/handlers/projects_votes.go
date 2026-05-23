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

type projectsRepo interface {
	List(ctx context.Context, opts repository.ListOpts) ([]*models.Project, int64, error)
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

type projectPhasesRepo interface {
	ListByProject(ctx context.Context, projectID string) ([]*models.ProjectPhase, error)
}

type votesRepo interface {
	ListByProject(ctx context.Context, projectID string, opts repository.ListOpts) ([]*models.Vote, int64, error)
}

// ProjectsList handles GET /api/v1/projects.
func ProjectsList(repo projectsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := httpx.ParsePagination(r)
		rows, total, err := repo.List(r.Context(), repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromProjects(rows), p.Page, p.PageSize, total))
	}
}

// ProjectsGet handles GET /api/v1/projects/{id}.
func ProjectsGet(repo projectsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_id", "id is required")
			return
		}
		p, err := repo.GetByID(r.Context(), id)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, dto.FromProject(p))
	}
}

// ProjectsPhases handles GET /api/v1/projects/{id}/phases. Returns
// every phase of the project, in ascending creation order.
func ProjectsPhases(repo projectPhasesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_id", "id is required")
			return
		}
		rows, err := repo.ListByProject(r.Context(), id)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"data": dto.FromProjectPhases(rows),
		})
	}
}

// ProjectsVotes handles GET /api/v1/projects/{id}/votes. Returns votes
// targeting either the project directly or any of its phases.
func ProjectsVotes(repo votesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			httpx.WriteProblem(w, http.StatusBadRequest, "invalid_id", "id is required")
			return
		}
		p := httpx.ParsePagination(r)
		rows, total, err := repo.ListByProject(r.Context(), id, repository.ListOpts{
			Limit: p.PageSize, Offset: p.Offset(),
		})
		if err != nil {
			writeRepoError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK,
			dto.NewPage(dto.FromVotes(rows), p.Page, p.PageSize, total))
	}
}
