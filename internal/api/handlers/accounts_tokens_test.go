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

type fakeAccountsRepo struct {
	byAddr map[string]*models.Account
}

func (f *fakeAccountsRepo) GetByAddress(_ context.Context, a string) (*models.Account, error) {
	if v, ok := f.byAddr[a]; ok {
		return v, nil
	}
	return nil, pgx.ErrNoRows
}

type fakeAccountBalancesRepo struct {
	byAddr map[string][]*models.Balance
}

func (f *fakeAccountBalancesRepo) ListByAddress(_ context.Context, a string) ([]*models.Balance, error) {
	return f.byAddr[a], nil
}

type fakeTokensRepo struct {
	list   []*models.Token
	total  int64
	byStd  map[string]*models.Token
	listOp repository.ListOpts
}

func (f *fakeTokensRepo) List(_ context.Context, o repository.ListOpts) ([]*models.Token, int64, error) {
	f.listOp = o
	return f.list, f.total, nil
}
func (f *fakeTokensRepo) GetByStandard(_ context.Context, s string) (*models.Token, error) {
	if v, ok := f.byStd[s]; ok {
		return v, nil
	}
	return nil, pgx.ErrNoRows
}

type fakeTokenHoldersRepo struct {
	rows   []*models.Balance
	total  int64
	lastOp repository.ListOpts
	lastTS string
}

func (f *fakeTokenHoldersRepo) ListByToken(_ context.Context, ts string, o repository.ListOpts) ([]*models.Balance, int64, error) {
	f.lastTS = ts
	f.lastOp = o
	return f.rows, f.total, nil
}

func TestAccountsGet(t *testing.T) {
	firstSeen := int64(1_700_000_000)
	lastSeen := int64(1_700_000_500)
	repo := &fakeAccountsRepo{byAddr: map[string]*models.Account{
		"z1qq": {
			Address:           "z1qq",
			BlockCount:        5,
			GenesisZnnBalance: 1_000_000_000_000,
			FirstSeen:         &firstSeen,
			LastSeen:          &lastSeen,
			TxCount:           42,
		},
		"z1empty": {Address: "z1empty"},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}", AccountsGet(repo))

	t.Run("ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, `"address":"z1qq"`) {
			t.Errorf("missing address in %s", body)
		}
		if !strings.Contains(body, `"genesis_znn_balance":"1000000000000"`) {
			t.Errorf("expected stringified amount in %s", body)
		}
		if !strings.Contains(body, `"first_seen":1700000000`) {
			t.Errorf("missing first_seen in %s", body)
		}
		if !strings.Contains(body, `"last_seen":1700000500`) {
			t.Errorf("missing last_seen in %s", body)
		}
		if !strings.Contains(body, `"tx_count":42`) {
			t.Errorf("missing tx_count in %s", body)
		}
	})

	t.Run("ok_null_seen", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1empty", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		body := w.Body.String()
		// Unseen address: first_seen/last_seen must serialize as JSON null,
		// not be omitted (the spec marks them required).
		if !strings.Contains(body, `"first_seen":null`) {
			t.Errorf("expected first_seen:null in %s", body)
		}
		if !strings.Contains(body, `"last_seen":null`) {
			t.Errorf("expected last_seen:null in %s", body)
		}
		if !strings.Contains(body, `"tx_count":0`) {
			t.Errorf("expected tx_count:0 in %s", body)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1unknown", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestAccountsBalances(t *testing.T) {
	repo := &fakeAccountBalancesRepo{byAddr: map[string][]*models.Balance{
		"z1qq": {
			{Address: "z1qq", TokenStandard: "zts1znn", Balance: 100},
			{Address: "z1qq", TokenStandard: "zts1qsr", Balance: 200},
		},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/accounts/{address}/balances", AccountsBalances(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1qq/balances", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"balance":"200"`) {
		t.Errorf("missing balance in %s", w.Body.String())
	}

	// Empty address bucket returns [] not null.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/accounts/z1empty/balances", nil))
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("expected data:[] in %s", w.Body.String())
	}
}

func TestTokensList(t *testing.T) {
	repo := &fakeTokensRepo{
		list:  []*models.Token{{TokenStandard: "zts1znn", Name: "ZNN", Symbol: "ZNN"}},
		total: 1,
	}
	w := httptest.NewRecorder()
	TokensList(repo)(w, httptest.NewRequest(http.MethodGet, "/api/v1/tokens?page=1&page_size=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.listOp.Limit != 10 || repo.listOp.Offset != 0 {
		t.Errorf("listOp = %+v", repo.listOp)
	}
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Errorf("missing pagination in %s", w.Body.String())
	}
}

func TestTokensGet(t *testing.T) {
	repo := &fakeTokensRepo{byStd: map[string]*models.Token{
		"zts1znn": {TokenStandard: "zts1znn", Symbol: "ZNN", TotalSupply: 9_000_000_000_000_000_00},
	}}
	r := chi.NewRouter()
	r.Get("/api/v1/tokens/{token_standard}", TokensGet(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/tokens/zts1znn", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// Confirms the > 2^53 amount survives the round-trip as a string.
	if !strings.Contains(w.Body.String(), `"total_supply":"900000000000000000"`) {
		t.Errorf("missing stringified total_supply in %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/tokens/zts1missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestTokensHolders(t *testing.T) {
	repo := &fakeTokenHoldersRepo{
		rows:  []*models.Balance{{Address: "z1qq", TokenStandard: "zts1znn", Balance: 5}},
		total: 1,
	}
	r := chi.NewRouter()
	r.Get("/api/v1/tokens/{token_standard}/holders", TokensHolders(repo))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/tokens/zts1znn/holders?page=2&page_size=20", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.lastTS != "zts1znn" || repo.lastOp.Limit != 20 || repo.lastOp.Offset != 20 {
		t.Errorf("repo state = %q %+v", repo.lastTS, repo.lastOp)
	}
}
