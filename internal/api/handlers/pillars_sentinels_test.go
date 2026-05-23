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

type fakePillarsRepo struct {
	list                []*models.Pillar
	total               int64
	byName              map[string]*models.Pillar
	delegators          []*repository.PillarDelegator
	delTotal            int64
	lastIncludeRevoked  bool
	lastListOp          repository.ListOpts
	lastDelegatorsOwner string
}

func (f *fakePillarsRepo) List(_ context.Context, includeRevoked bool, o repository.ListOpts) ([]*models.Pillar, int64, error) {
	f.lastIncludeRevoked = includeRevoked
	f.lastListOp = o
	return f.list, f.total, nil
}
func (f *fakePillarsRepo) GetByName(_ context.Context, n string) (*models.Pillar, error) {
	if v, ok := f.byName[n]; ok {
		return v, nil
	}
	return nil, pgx.ErrNoRows
}
func (f *fakePillarsRepo) ListDelegators(_ context.Context, owner string, o repository.ListOpts) ([]*repository.PillarDelegator, int64, error) {
	f.lastDelegatorsOwner = owner
	_ = o
	return f.delegators, f.delTotal, nil
}

func TestPillarsList(t *testing.T) {
	repo := &fakePillarsRepo{
		list: []*models.Pillar{
			{Name: "alphanet-1", OwnerAddress: "z1qp1", Rank: 0, Weight: 1_000},
		},
		total: 1,
	}
	w := httptest.NewRecorder()
	PillarsList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/pillars?include_revoked=true", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !repo.lastIncludeRevoked {
		t.Errorf("expected include_revoked=true to propagate")
	}
	if !strings.Contains(w.Body.String(), `"weight":"1000"`) {
		t.Errorf("missing stringified weight in %s", w.Body.String())
	}
}

func TestPillarsGetByName(t *testing.T) {
	repo := &fakePillarsRepo{byName: map[string]*models.Pillar{
		"alphanet-1": {Name: "alphanet-1", OwnerAddress: "z1qp1"},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/pillars/{name}", PillarsGetByName(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/pillars/alphanet-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/pillars/unknown", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestPillarsDelegators(t *testing.T) {
	repo := &fakePillarsRepo{
		byName: map[string]*models.Pillar{
			"alphanet-1": {Name: "alphanet-1", OwnerAddress: "z1qp1"},
		},
		delegators: []*repository.PillarDelegator{
			{Address: "z1qd1", DelegationStartTimestamp: 1700000000},
		},
		delTotal: 1,
	}
	r := chi.NewRouter()
	r.Get("/api/v1/pillars/{name}/delegators", PillarsDelegators(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/pillars/alphanet-1/delegators", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.lastDelegatorsOwner != "z1qp1" {
		t.Errorf("expected delegators owner = z1qp1, got %q", repo.lastDelegatorsOwner)
	}

	// Unknown pillar name → 404 from the GetByName lookup, not from ListDelegators.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/pillars/unknown/delegators", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

type fakeSentinelsRepo struct {
	rows           []*models.Sentinel
	total          int64
	lastActiveOnly bool
}

func (f *fakeSentinelsRepo) List(_ context.Context, activeOnly bool, _ repository.ListOpts) ([]*models.Sentinel, int64, error) {
	f.lastActiveOnly = activeOnly
	return f.rows, f.total, nil
}

func TestSentinelsList(t *testing.T) {
	repo := &fakeSentinelsRepo{
		rows:  []*models.Sentinel{{Owner: "z1qs1", Active: true}},
		total: 1,
	}
	t.Run("default_active_only", func(t *testing.T) {
		w := httptest.NewRecorder()
		SentinelsList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/sentinels", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if !repo.lastActiveOnly {
			t.Errorf("expected default activeOnly=true")
		}
	})
	t.Run("include_inactive", func(t *testing.T) {
		w := httptest.NewRecorder()
		SentinelsList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/sentinels?include_inactive=true", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if repo.lastActiveOnly {
			t.Errorf("expected include_inactive=true to disable activeOnly filter")
		}
	})
}
