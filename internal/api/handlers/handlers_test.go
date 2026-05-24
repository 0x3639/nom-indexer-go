package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// fakeMomentumRepo satisfies both statusMomentumRepo and momentumsRepo.
type fakeMomentumRepo struct {
	latest     *models.Momentum
	latestErr  error
	byHeight   map[uint64]*models.Momentum
	listResult []*models.Momentum
	listTotal  int64
	listErr    error
	lastOpts   repository.ListOpts
}

func (f *fakeMomentumRepo) GetLatest(_ context.Context) (*models.Momentum, error) {
	return f.latest, f.latestErr
}
func (f *fakeMomentumRepo) GetByHeight(_ context.Context, h uint64) (*models.Momentum, error) {
	if m, ok := f.byHeight[h]; ok {
		return m, nil
	}
	return nil, pgx.ErrNoRows
}
func (f *fakeMomentumRepo) List(_ context.Context, opts repository.ListOpts) ([]*models.Momentum, int64, error) {
	f.lastOpts = opts
	return f.listResult, f.listTotal, f.listErr
}

func TestStatus_OK(t *testing.T) {
	repo := &fakeMomentumRepo{
		latest: &models.Momentum{Height: 100, Timestamp: 1700000000},
	}
	now := func() time.Time { return time.Unix(1700000042, 0) }

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	Status(repo, "v9.9.9", now)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	var got dto.Status
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.LatestHeight != 100 || got.LatestTimestamp != 1700000000 ||
		got.IndexerLagSeconds != 42 || got.Version != "v9.9.9" {
		t.Errorf("status response = %+v", got)
	}
}

func TestStatus_EmptyTable(t *testing.T) {
	repo := &fakeMomentumRepo{latestErr: pgx.ErrNoRows}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	Status(repo, "dev", time.Now)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"latest_height":0`) {
		t.Errorf("empty-table body = %s", w.Body.String())
	}
}

func TestStatus_DBError(t *testing.T) {
	repo := &fakeMomentumRepo{latestErr: errors.New("connection refused")}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	Status(repo, "dev", time.Now)(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
}

func TestMomentumsList_Envelope(t *testing.T) {
	repo := &fakeMomentumRepo{
		listResult: []*models.Momentum{
			{Height: 2, Hash: "b"},
			{Height: 1, Hash: "a"},
		},
		listTotal: 25,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/momentums?page=2&page_size=10", nil)
	MomentumsList(repo)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.lastOpts.Limit != 10 || repo.lastOpts.Offset != 10 || repo.lastOpts.Sort != "desc" {
		t.Errorf("opts passed to repo = %+v", repo.lastOpts)
	}
	var got struct {
		Data       []dto.Momentum `json:"data"`
		Pagination dto.Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 || got.Pagination.Page != 2 || got.Pagination.PageSize != 10 || got.Pagination.Total != 25 {
		t.Errorf("envelope = %+v", got)
	}
}

func TestMomentumsList_SortAsc(t *testing.T) {
	repo := &fakeMomentumRepo{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/momentums?sort=asc", nil)
	MomentumsList(repo)(w, r)
	if repo.lastOpts.Sort != "asc" {
		t.Errorf("sort = %q, want asc", repo.lastOpts.Sort)
	}
}

func TestMomentumsList_EmptyDataNotNull(t *testing.T) {
	repo := &fakeMomentumRepo{} // nil listResult
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/momentums", nil)
	MomentumsList(repo)(w, r)
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("expected data:[] in %s", w.Body.String())
	}
}

func TestMomentumsLatest_OK(t *testing.T) {
	repo := &fakeMomentumRepo{latest: &models.Momentum{Height: 99, Hash: "h99"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/momentums/latest", nil)
	MomentumsLatest(repo)(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"height":99`) {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMomentumsLatest_Empty(t *testing.T) {
	repo := &fakeMomentumRepo{latestErr: pgx.ErrNoRows}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/momentums/latest", nil)
	MomentumsLatest(repo)(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestMomentumsGetByHeight exercises path-param parsing; we wire chi so
// URLParam works.
func TestMomentumsGetByHeight(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		repo    *fakeMomentumRepo
		wantSts int
	}{
		{
			name:    "ok",
			path:    "/api/v1/momentums/7",
			repo:    &fakeMomentumRepo{byHeight: map[uint64]*models.Momentum{7: {Height: 7, Hash: "h7"}}},
			wantSts: http.StatusOK,
		},
		{
			name:    "not_found",
			path:    "/api/v1/momentums/999",
			repo:    &fakeMomentumRepo{byHeight: map[uint64]*models.Momentum{}},
			wantSts: http.StatusNotFound,
		},
		{
			name:    "bad_height",
			path:    "/api/v1/momentums/abc",
			repo:    &fakeMomentumRepo{},
			wantSts: http.StatusBadRequest,
		},
		{
			name:    "negative_height",
			path:    "/api/v1/momentums/-1",
			repo:    &fakeMomentumRepo{},
			wantSts: http.StatusBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/api/v1/momentums/{height}", MomentumsGetByHeight(tc.repo))
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.wantSts {
				t.Errorf("status = %d, want %d (body=%s)", w.Code, tc.wantSts, w.Body.String())
			}
		})
	}
}
