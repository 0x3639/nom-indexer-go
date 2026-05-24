package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

type fakeBridgeRepo struct {
	wraps          []*models.WrapTokenRequest
	unwraps        []*models.UnwrapTokenRequest
	wrapsTotal     int64
	unwrapsTotal   int64
	wrapsByAddr    []*models.WrapTokenRequest
	unwrapsByAddr  []*models.UnwrapTokenRequest
	lastWrapAddr   string
	lastUnwrapAddr string
	lastWrapsOp    repository.ListOpts
}

func (f *fakeBridgeRepo) ListWraps(_ context.Context, o repository.ListOpts) ([]*models.WrapTokenRequest, int64, error) {
	f.lastWrapsOp = o
	return f.wraps, f.wrapsTotal, nil
}
func (f *fakeBridgeRepo) ListWrapsByAddress(_ context.Context, addr string, _ repository.ListOpts) ([]*models.WrapTokenRequest, int64, error) {
	f.lastWrapAddr = addr
	return f.wrapsByAddr, 0, nil
}
func (f *fakeBridgeRepo) ListUnwraps(_ context.Context, _ repository.ListOpts) ([]*models.UnwrapTokenRequest, int64, error) {
	return f.unwraps, f.unwrapsTotal, nil
}
func (f *fakeBridgeRepo) ListUnwrapsByAddress(_ context.Context, addr string, _ repository.ListOpts) ([]*models.UnwrapTokenRequest, int64, error) {
	f.lastUnwrapAddr = addr
	return f.unwrapsByAddr, 0, nil
}

func TestBridgeWraps(t *testing.T) {
	repo := &fakeBridgeRepo{wraps: []*models.WrapTokenRequest{{ID: "w1", Amount: 1_000_000_000_000}}, wrapsTotal: 1}
	w := httptest.NewRecorder()
	BridgeWraps(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/bridge/wraps", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"amount":"1000000000000"`) {
		t.Errorf("missing amount in %s", w.Body.String())
	}
}

func TestBridgeUnwraps(t *testing.T) {
	repo := &fakeBridgeRepo{unwraps: []*models.UnwrapTokenRequest{{TransactionHash: "0xdead"}}, unwrapsTotal: 1}
	w := httptest.NewRecorder()
	BridgeUnwraps(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/bridge/unwraps", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"transaction_hash":"0xdead"`) {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBridgeByAddress(t *testing.T) {
	repo := &fakeBridgeRepo{
		wrapsByAddr:   []*models.WrapTokenRequest{{ID: "w1"}},
		unwrapsByAddr: []*models.UnwrapTokenRequest{{TransactionHash: "0xfeed"}},
	}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/bridge/wraps", BridgeWrapsByAddress(repo))
	r.Get("/api/v1/accounts/{address}/bridge/unwraps", BridgeUnwrapsByAddress(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/bridge/wraps", nil))
	if w.Code != http.StatusOK || repo.lastWrapAddr != "z1qq" {
		t.Errorf("wraps state: code=%d addr=%q", w.Code, repo.lastWrapAddr)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/bridge/unwraps", nil))
	if w.Code != http.StatusOK || repo.lastUnwrapAddr != "z1qq" {
		t.Errorf("unwraps state: code=%d addr=%q", w.Code, repo.lastUnwrapAddr)
	}
}
