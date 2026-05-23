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

type fakeAccountBlocksRepo struct {
	list, byAddrList []*models.AccountBlock
	total            int64
	byHash           map[string]*models.AccountBlock
	listOp           repository.ListOpts
	byAddrAddr       string
	byAddrOp         repository.ListOpts
}

func (f *fakeAccountBlocksRepo) List(_ context.Context, o repository.ListOpts) ([]*models.AccountBlock, int64, error) {
	f.listOp = o
	return f.list, f.total, nil
}
func (f *fakeAccountBlocksRepo) ListByAddress(_ context.Context, a string, o repository.ListOpts) ([]*models.AccountBlock, int64, error) {
	f.byAddrAddr = a
	f.byAddrOp = o
	return f.byAddrList, f.total, nil
}
func (f *fakeAccountBlocksRepo) GetByHash(_ context.Context, h string) (*models.AccountBlock, error) {
	if v, ok := f.byHash[h]; ok {
		return v, nil
	}
	return nil, pgx.ErrNoRows
}

func TestAccountBlocksList(t *testing.T) {
	repo := &fakeAccountBlocksRepo{
		list:  []*models.AccountBlock{{Hash: "abc", Amount: 12345}},
		total: 1,
	}
	w := httptest.NewRecorder()
	AccountBlocksList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/account_blocks?page=1&page_size=10&sort=asc", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.listOp.Sort != "asc" {
		t.Errorf("sort = %q", repo.listOp.Sort)
	}
	if !strings.Contains(w.Body.String(), `"amount":"12345"`) {
		t.Errorf("missing stringified amount in %s", w.Body.String())
	}
}

func TestAccountBlocksGet(t *testing.T) {
	repo := &fakeAccountBlocksRepo{byHash: map[string]*models.AccountBlock{
		"abc": {Hash: "abc"},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/account_blocks/{hash}", AccountBlocksGet(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/account_blocks/abc", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/account_blocks/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestAccountBlocksByAddress(t *testing.T) {
	repo := &fakeAccountBlocksRepo{
		byAddrList: []*models.AccountBlock{{Hash: "xyz", Address: "z1qq"}},
		total:      1,
	}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/transactions", AccountBlocksByAddress(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/transactions?page=3", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.byAddrAddr != "z1qq" || repo.byAddrOp.Offset != 100 {
		t.Errorf("repo state = %q %+v", repo.byAddrAddr, repo.byAddrOp)
	}
}
