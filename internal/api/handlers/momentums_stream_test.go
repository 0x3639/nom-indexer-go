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

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

// fakeStreamRepo satisfies streamMomentumsRepo for replay tests.
type fakeStreamRepo struct {
	latest *models.Momentum
	byH    map[uint64]*models.Momentum
}

func (f *fakeStreamRepo) GetLatest(_ context.Context) (*models.Momentum, error) {
	if f.latest == nil {
		return nil, pgx.ErrNoRows
	}
	return f.latest, nil
}

func (f *fakeStreamRepo) ListByHeightRange(_ context.Context, from, to uint64, limit int) ([]*models.Momentum, error) {
	var out []*models.Momentum
	for h := from; h <= to; h++ {
		if m, ok := f.byH[h]; ok {
			out = append(out, m)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func newStreamHarness(t *testing.T, repo *fakeStreamRepo) (string, *auth.Signer, *stream.Hub[*dto.Momentum], func()) {
	t.Helper()
	signer, err := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	hub := stream.New(stream.Config[*dto.Momentum]{
		Logger:      zap.NewNop(),
		ChannelName: "momentum_new",
		Unmarshal:   stream.UnmarshalJSON[dto.Momentum](),
	})
	stream.MarkRunningForTest(hub) // bypass the LISTEN loop for handler tests

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/momentums/stream", MomentumsStream(signer, hub, repo))
	srv := httptest.NewServer(mux)
	// httptest.Server uses http:// — swap to ws:// for the WS dialer.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/momentums/stream"
	return wsURL, signer, hub, srv.Close
}

func TestStream_RejectsMissingAuth(t *testing.T) {
	wsURL, _, _, cleanup := newStreamHarness(t, &fakeStreamRepo{})
	defer cleanup()

	httpURL := strings.Replace(wsURL, "ws://", "http://", 1)
	resp := mustGet(t, httpURL)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http GET %s: %v", url, err)
	}
	return resp
}

func TestStream_AcceptsTokenQueryParam(t *testing.T) {
	wsURL, signer, hub, cleanup := newStreamHarness(t, &fakeStreamRepo{})
	defer cleanup()

	tok, _ := signer.Issue("browser-client", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"?token="+tok, nil)
	if err != nil {
		t.Fatalf("ws dial with ?token=: %v", err)
	}
	defer conn.CloseNow()

	// Push one momentum through the hub; expect to read it as a frame.
	want := &dto.Momentum{Height: 99, Hash: "h99", Producer: "z1p"}
	go func() {
		// Give the handler a moment to subscribe.
		time.Sleep(20 * time.Millisecond)
		// Internal dispatch helper isn't exported; emulate by calling
		// Subscribe + sending… but the handler's subscriber is private.
		// Instead, just call hub.dispatch via reflection? Simpler:
		// reach in through the package-internal API.
		hubDispatch(hub, want)
	}()

	_, body, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var got dto.Momentum
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if got.Height != 99 || got.Hash != "h99" {
		t.Errorf("frame = %+v, want height=99 hash=h99", got)
	}
}

func TestStream_AcceptsAuthorizationHeader(t *testing.T) {
	wsURL, signer, hub, cleanup := newStreamHarness(t, &fakeStreamRepo{})
	defer cleanup()

	tok, _ := signer.Issue("server-client", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + tok}},
	})
	if err != nil {
		t.Fatalf("ws dial with header: %v", err)
	}
	defer conn.CloseNow()

	want := &dto.Momentum{Height: 7, Hash: "h7", Producer: "z1p"}
	go func() {
		time.Sleep(20 * time.Millisecond)
		hubDispatch(hub, want)
	}()

	_, body, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var got dto.Momentum
	_ = json.Unmarshal(body, &got)
	if got.Height != 7 {
		t.Errorf("frame height = %d, want 7", got.Height)
	}
}

func TestStream_ReplayFromHeight(t *testing.T) {
	repo := &fakeStreamRepo{
		latest: &models.Momentum{Height: 10, Hash: "h10"},
		byH: map[uint64]*models.Momentum{
			8:  {Height: 8, Hash: "h8"},
			9:  {Height: 9, Hash: "h9"},
			10: {Height: 10, Hash: "h10"},
		},
	}
	wsURL, signer, _, cleanup := newStreamHarness(t, repo)
	defer cleanup()

	tok, _ := signer.Issue("replay", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"?token="+tok+"&from_height=8", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.CloseNow()

	// Expect 3 frames in order: 8, 9, 10.
	for _, wantH := range []uint64{8, 9, 10} {
		_, body, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read height %d: %v", wantH, err)
		}
		var m dto.Momentum
		_ = json.Unmarshal(body, &m)
		if m.Height != wantH {
			t.Errorf("got height %d, want %d", m.Height, wantH)
		}
	}
}

func TestStream_InvalidFromHeight(t *testing.T) {
	wsURL, signer, _, cleanup := newStreamHarness(t, &fakeStreamRepo{})
	defer cleanup()
	tok, _ := signer.Issue("bad", time.Hour, []string{"read"})

	httpURL := strings.Replace(wsURL, "ws://", "http://", 1) + "?token=" + tok + "&from_height=notanumber"
	resp := mustGet(t, httpURL)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStream_ServerShutdownClosesClients(t *testing.T) {
	wsURL, signer, _, cleanup := newStreamHarness(t, &fakeStreamRepo{})
	tok, _ := signer.Issue("ephemeral", time.Hour, []string{"read"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"?token="+tok, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.CloseNow()

	cleanup() // closes the http server, drops the underlying TCP
	// Read should return some error (peer reset, EOF, or close frame).
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Error("expected read error after server shutdown")
	}
	// Don't assert the specific error: kernel-level peer reset vs WS
	// close frame depends on shutdown timing.
	_ = errors.Is // imports keepalive
}

// TestStream_HubNotRunningReturns503 covers the case where the LISTEN
// loop has not started (or has exited). The handler must surface the
// hub state as a plain HTTP 503 problem-detail, NOT as a successful
// upgrade that hangs forever.
func TestStream_HubNotRunningReturns503(t *testing.T) {
	signer, _ := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	hub := stream.New(stream.Config[*dto.Momentum]{
		Logger:      zap.NewNop(),
		ChannelName: "momentum_new",
		Unmarshal:   stream.UnmarshalJSON[dto.Momentum](),
	})
	// Deliberately do NOT call MarkRunningForTest — hub is pending.

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/momentums/stream", MomentumsStream(signer, hub, &fakeStreamRepo{}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tok, _ := signer.Issue("anyone", time.Hour, []string{"read"})
	resp := mustGet(t, srv.URL+"/api/v1/momentums/stream?token="+tok)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// TestStream_PerSubjectLimitReturns429 confirms that once a JWT
// subject hits the hub's per-subject cap, further upgrade requests
// are rejected with 429 instead of being accepted.
func TestStream_PerSubjectLimitReturns429(t *testing.T) {
	signer, _ := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	hub := stream.New(stream.Config[*dto.Momentum]{
		Logger:        zap.NewNop(),
		ChannelName:   "momentum_new",
		Unmarshal:     stream.UnmarshalJSON[dto.Momentum](),
		PerSubjectMax: 2,
	})
	stream.MarkRunningForTest(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/momentums/stream", MomentumsStream(signer, hub, &fakeStreamRepo{}))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/momentums/stream"

	tok, _ := signer.Issue("greedy", time.Hour, []string{"read"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First two opens succeed.
	var conns []*websocket.Conn
	defer func() {
		for _, c := range conns {
			c.CloseNow()
		}
	}()
	for i := 0; i < 2; i++ {
		c, _, err := websocket.Dial(ctx, wsURL+"?token="+tok, nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		conns = append(conns, c)
	}

	// Third one must be rejected with 429 from the HTTP upgrade.
	resp := mustGet(t, srv.URL+"/api/v1/momentums/stream?token="+tok)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
}

// hubDispatch synthesizes a NOTIFY without the postgres round-trip.
// The stream package exposes DispatchForTest as the only way to inject
// a momentum into the in-process fan-out from outside the package.
func hubDispatch(h *stream.Hub[*dto.Momentum], m *dto.Momentum) {
	stream.DispatchForTest(h, m)
}
