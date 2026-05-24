package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

// fakeTxRepo + fakeTxMomentumRepo satisfy the transactions stream
// handler's two repository interfaces. Pre-seeded blocks are returned
// by ListByMomentumHeightRange in ASC order of momentum_height, with
// the optional address filter applied (mirrors the real SQL semantics).
type fakeTxRepo struct {
	rows []*models.AccountBlock
}

func (f *fakeTxRepo) ListByMomentumHeightRange(
	_ context.Context,
	from, to int64,
	addr string,
	limit int,
) ([]*models.AccountBlock, error) {
	out := make([]*models.AccountBlock, 0, len(f.rows))
	for _, ab := range f.rows {
		if ab.MomentumHeight < from || ab.MomentumHeight > to {
			continue
		}
		if addr != "" && ab.Address != addr && ab.ToAddress != addr {
			continue
		}
		out = append(out, ab)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type fakeTxMomentumRepo struct {
	latest *models.Momentum
}

func (f *fakeTxMomentumRepo) GetLatest(_ context.Context) (*models.Momentum, error) {
	if f.latest == nil {
		return nil, pgx.ErrNoRows
	}
	return f.latest, nil
}

func newTxStreamHarness(
	t *testing.T, txRepo *fakeTxRepo, mom *fakeTxMomentumRepo,
) (wsURL string, signer *auth.Signer, hub *stream.Hub[*dto.AccountBlock], cleanup func()) {
	t.Helper()
	signer, err := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	hub = stream.New(stream.Config[*dto.AccountBlock]{
		Logger:      zap.NewNop(),
		ChannelName: "account_block_new",
		Unmarshal:   stream.UnmarshalJSON[dto.AccountBlock](),
	})
	stream.MarkRunningForTest(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/transactions/stream",
		TransactionsStream(signer, hub, txRepo, mom))
	srv := httptest.NewServer(mux)
	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/transactions/stream"
	return wsURL, signer, hub, srv.Close
}

func TestTxStream_LiveFirehose(t *testing.T) {
	wsURL, signer, hub, cleanup := newTxStreamHarness(t, &fakeTxRepo{}, &fakeTxMomentumRepo{})
	defer cleanup()

	tok, _ := signer.Issue("firehose", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + tok}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// Push two blocks via the hub; both should arrive.
	go func() {
		time.Sleep(20 * time.Millisecond)
		stream.DispatchForTest(hub, &dto.AccountBlock{
			Hash: "ab-1", MomentumHeight: 100, Address: "z1qsender", ToAddress: "z1qrecv", Amount: "1",
		})
		stream.DispatchForTest(hub, &dto.AccountBlock{
			Hash: "ab-2", MomentumHeight: 101, Address: "z1qother", ToAddress: "z1qrecv2", Amount: "2",
		})
	}()

	for _, want := range []string{"ab-1", "ab-2"} {
		_, body, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		var got dto.AccountBlock
		_ = json.Unmarshal(body, &got)
		if got.Hash != want {
			t.Errorf("got %q, want %q", got.Hash, want)
		}
	}
}

func TestTxStream_AddressFilterDropsNonMatchingFrames(t *testing.T) {
	wsURL, signer, hub, cleanup := newTxStreamHarness(t, &fakeTxRepo{}, &fakeTxMomentumRepo{})
	defer cleanup()

	tok, _ := signer.Issue("addr-filter", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"?token="+tok+"&address=z1qme", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	go func() {
		time.Sleep(20 * time.Millisecond)
		// Block 1: irrelevant, should be filtered out.
		stream.DispatchForTest(hub, &dto.AccountBlock{
			Hash: "ab-skip", MomentumHeight: 100,
			Address: "z1qother", ToAddress: "z1qother2",
		})
		// Block 2: sender matches → pass.
		stream.DispatchForTest(hub, &dto.AccountBlock{
			Hash: "ab-sender", MomentumHeight: 101,
			Address: "z1qme", ToAddress: "z1qrecv",
		})
		// Block 3: recipient matches → pass.
		stream.DispatchForTest(hub, &dto.AccountBlock{
			Hash: "ab-recv", MomentumHeight: 102,
			Address: "z1qother", ToAddress: "z1qme",
		})
	}()

	for _, want := range []string{"ab-sender", "ab-recv"} {
		_, body, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		var got dto.AccountBlock
		_ = json.Unmarshal(body, &got)
		if got.Hash != want {
			t.Errorf("got %q, want %q (ab-skip should never have been written)", got.Hash, want)
		}
	}
}

func TestTxStream_ReplayThenLive(t *testing.T) {
	mom := &fakeTxMomentumRepo{latest: &models.Momentum{Height: 105}}
	txRepo := &fakeTxRepo{
		rows: []*models.AccountBlock{
			{Hash: "h100", MomentumHeight: 100, Address: "z1qa"},
			{Hash: "h101", MomentumHeight: 101, Address: "z1qa"},
			{Hash: "h105", MomentumHeight: 105, Address: "z1qa"},
		},
	}
	wsURL, signer, hub, cleanup := newTxStreamHarness(t, txRepo, mom)
	defer cleanup()
	tok, _ := signer.Issue("replay-tx", time.Hour, []string{"read"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL+"?token="+tok+"&from_height=100", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// First three frames are the replay (sorted ASC by momentum_height).
	for _, want := range []string{"h100", "h101", "h105"} {
		_, body, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		var got dto.AccountBlock
		_ = json.Unmarshal(body, &got)
		if got.Hash != want {
			t.Errorf("got %q, want %q", got.Hash, want)
		}
	}

	// Live dispatch at momentum_height 106 should arrive next; a dup at
	// 105 (already replayed) must be suppressed.
	go func() {
		time.Sleep(20 * time.Millisecond)
		stream.DispatchForTest(hub, &dto.AccountBlock{Hash: "h105-dup", MomentumHeight: 105})
		stream.DispatchForTest(hub, &dto.AccountBlock{Hash: "h106", MomentumHeight: 106})
	}()
	_, body, readErr := conn.Read(ctx)
	if readErr != nil {
		t.Fatalf("read live: %v", readErr)
	}
	var live dto.AccountBlock
	_ = json.Unmarshal(body, &live)
	if live.Hash != "h106" {
		t.Errorf("got %q, want h106 (h105-dup should have been suppressed)", live.Hash)
	}
}

func TestTxStream_InvalidFromHeightReturns400(t *testing.T) {
	wsURL, signer, _, cleanup := newTxStreamHarness(t, &fakeTxRepo{}, &fakeTxMomentumRepo{})
	defer cleanup()
	tok, _ := signer.Issue("bad", time.Hour, []string{"read"})

	httpURL := strings.Replace(wsURL, "ws://", "http://", 1) +
		"?token=" + tok + "&from_height=garbage"
	resp := mustGet(t, httpURL)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
