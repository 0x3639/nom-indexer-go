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

type fakeStakesRepo struct {
	rows         []*models.Stake
	total        int64
	lastActive   bool
	lastAddrArg  string
	byAddrRows   []*models.Stake
	byAddrTotal  int64
	byAddrActive bool
}

func (f *fakeStakesRepo) List(_ context.Context, activeOnly bool, _ repository.ListOpts) ([]*models.Stake, int64, error) {
	f.lastActive = activeOnly
	return f.rows, f.total, nil
}
func (f *fakeStakesRepo) ListByAddress(_ context.Context, addr string, activeOnly bool, _ repository.ListOpts) ([]*models.Stake, int64, error) {
	f.lastAddrArg = addr
	f.byAddrActive = activeOnly
	return f.byAddrRows, f.byAddrTotal, nil
}

func TestStakesList(t *testing.T) {
	repo := &fakeStakesRepo{rows: []*models.Stake{{ID: "s1", ZnnAmount: 9_223_372_036_000_000_000}}, total: 1}
	w := httptest.NewRecorder()
	StakesList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/stakes", nil))
	if w.Code != http.StatusOK || !repo.lastActive {
		t.Fatalf("status=%d activeOnly=%v", w.Code, repo.lastActive)
	}
	if !strings.Contains(w.Body.String(), `"znn_amount":"9223372036000000000"`) {
		t.Errorf("missing stringified amount in %s", w.Body.String())
	}
}

func TestStakesByAddress(t *testing.T) {
	repo := &fakeStakesRepo{byAddrRows: []*models.Stake{{ID: "s1", Address: "z1qq"}}, byAddrTotal: 1}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/stakes", StakesByAddress(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/stakes?include_inactive=true", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.lastAddrArg != "z1qq" || repo.byAddrActive {
		t.Errorf("state: addr=%q active=%v", repo.lastAddrArg, repo.byAddrActive)
	}
}

type fakeFusionsRepo struct {
	rows       []*models.Fusion
	total      int64
	lastActive bool
	lastAddr   string
	byAddrRows []*models.Fusion
}

func (f *fakeFusionsRepo) List(_ context.Context, activeOnly bool, _ repository.ListOpts) ([]*models.Fusion, int64, error) {
	f.lastActive = activeOnly
	return f.rows, f.total, nil
}
func (f *fakeFusionsRepo) ListByAddress(_ context.Context, addr string, _ bool, _ repository.ListOpts) ([]*models.Fusion, int64, error) {
	f.lastAddr = addr
	return f.byAddrRows, 0, nil
}

func TestFusionsList(t *testing.T) {
	repo := &fakeFusionsRepo{rows: []*models.Fusion{{ID: "f1"}}, total: 1}
	w := httptest.NewRecorder()
	FusionsList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/fusions", nil))
	if w.Code != http.StatusOK || !repo.lastActive {
		t.Fatalf("status=%d active=%v", w.Code, repo.lastActive)
	}
}

func TestFusionsByAddress(t *testing.T) {
	repo := &fakeFusionsRepo{byAddrRows: []*models.Fusion{{ID: "f1", Address: "z1qq"}}}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/fusions", FusionsByAddress(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/fusions", nil))
	if w.Code != http.StatusOK || repo.lastAddr != "z1qq" {
		t.Fatalf("status=%d addr=%q", w.Code, repo.lastAddr)
	}
}

type fakeRewardsRepo struct {
	cum        []*models.CumulativeReward
	hist       []*models.RewardTransaction
	histTotal  int64
	lastAddr   string
	lastHistOp repository.ListOpts
}

func (f *fakeRewardsRepo) CumulativeByAddress(_ context.Context, addr string) ([]*models.CumulativeReward, error) {
	f.lastAddr = addr
	return f.cum, nil
}
func (f *fakeRewardsRepo) HistoryByAddress(_ context.Context, addr string, o repository.ListOpts) ([]*models.RewardTransaction, int64, error) {
	f.lastAddr = addr
	f.lastHistOp = o
	return f.hist, f.histTotal, nil
}

func TestRewardsCumulative(t *testing.T) {
	repo := &fakeRewardsRepo{cum: []*models.CumulativeReward{
		{Address: "z1qq", RewardType: models.RewardTypeDelegation, Amount: 500, TokenStandard: "zts1znn"},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/rewards/cumulative", RewardsCumulative(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/rewards/cumulative", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"reward_type":"Delegation"`) {
		t.Errorf("missing reward_type enum string in %s", body)
	}
}

func TestRewardsHistory(t *testing.T) {
	repo := &fakeRewardsRepo{
		hist:      []*models.RewardTransaction{{Hash: "abc", Address: "z1qq", RewardType: models.RewardTypePillar}},
		histTotal: 7,
	}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/rewards", RewardsHistory(repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/rewards?page=1&page_size=5", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.lastHistOp.Limit != 5 || !strings.Contains(w.Body.String(), `"total":7`) {
		t.Errorf("state op=%+v body=%s", repo.lastHistOp, w.Body.String())
	}
}
