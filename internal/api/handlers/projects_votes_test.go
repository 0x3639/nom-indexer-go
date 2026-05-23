package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

type fakeProjectsRepo struct {
	list  []*models.Project
	total int64
	byID  map[string]*models.Project
}

func (f *fakeProjectsRepo) List(_ context.Context, _ repository.ListOpts) ([]*models.Project, int64, error) {
	return f.list, f.total, nil
}
func (f *fakeProjectsRepo) GetByID(_ context.Context, id string) (*models.Project, error) {
	if v, ok := f.byID[id]; ok {
		return v, nil
	}
	return nil, pgx.ErrNoRows
}

type fakeProjectPhasesRepo struct {
	rows []*models.ProjectPhase
}

func (f *fakeProjectPhasesRepo) ListByProject(_ context.Context, _ string) ([]*models.ProjectPhase, error) {
	return f.rows, nil
}

type fakeVotesRepo struct {
	rows  []*models.Vote
	total int64
}

func (f *fakeVotesRepo) ListByProject(_ context.Context, _ string, _ repository.ListOpts) ([]*models.Vote, int64, error) {
	return f.rows, f.total, nil
}

func TestProjectsList(t *testing.T) {
	repo := &fakeProjectsRepo{
		list:  []*models.Project{{ID: "p1", Name: "Beep", ZnnFundsNeeded: 1_000_000}},
		total: 1,
	}
	w := httptest.NewRecorder()
	ProjectsList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"znn_funds_needed":"1000000"`) {
		t.Errorf("missing stringified funds in %s", w.Body.String())
	}
}

func TestProjectsGet(t *testing.T) {
	repo := &fakeProjectsRepo{byID: map[string]*models.Project{"p1": {ID: "p1"}}}
	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}", ProjectsGet(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/projects/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestProjectsPhases(t *testing.T) {
	repo := &fakeProjectPhasesRepo{rows: []*models.ProjectPhase{{ID: "ph1", ProjectID: "p1"}}}
	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}/phases", ProjectsPhases(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/phases", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"id":"ph1"`) {
		t.Errorf("missing phase in %s", w.Body.String())
	}
}

func TestProjectsVotes(t *testing.T) {
	repo := &fakeVotesRepo{rows: []*models.Vote{{ID: 1, ProjectID: "p1", Vote: 1}}, total: 1}
	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}/votes", ProjectsVotes(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/votes", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"vote":1`) {
		t.Errorf("missing vote in %s", w.Body.String())
	}
}
