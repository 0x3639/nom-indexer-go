package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

// streamMomentumsRepo is the slice of MomentumRepository the stream
// handler needs for replay. Defined as an interface so tests can swap
// a fake without spinning up Postgres.
type streamMomentumsRepo interface {
	GetByHeight(ctx context.Context, height uint64) (*models.Momentum, error)
	GetLatest(ctx context.Context) (*models.Momentum, error)
}

// Tunables for the WebSocket connection. Sub-second precision matters
// here because momentums commit on a ~10 s cadence.
const (
	// streamWriteTimeout is the per-frame write deadline. WebSocket
	// frames are tiny; if the client TCP stack stalls beyond this we
	// assume the connection is gone and close it.
	streamWriteTimeout = 10 * time.Second

	// streamPingInterval is the heartbeat cadence. Without it, idle
	// proxies (NAT timeouts, HTTP/1.1 reverse proxies) drop the
	// connection in the 30-60 s range with no signal to either side.
	streamPingInterval = 25 * time.Second

	// streamReplayMaxRows is a hard ceiling on the catch-up scan so a
	// client requesting from_height=1 on mainnet doesn't pin the API
	// for minutes. Beyond this the connection switches to live mode
	// and the client can poll the REST API for the gap.
	streamReplayMaxRows = 10_000
)

// MomentumsStream handles GET /api/v1/momentums/stream as a WebSocket.
//
// Auth: Authorization: Bearer <token> header OR ?token=<jwt> query
// param fallback (browsers cannot send custom headers on a WS upgrade).
// Tokens in the query string can leak to proxy logs — minted JWTs
// should have short TTLs for stream consumers.
//
// Query parameters:
//   - from_height (optional, uint64): replay momentums starting at this
//     height before switching to live. Capped at streamReplayMaxRows
//     to bound the catch-up scan.
//
// Frames: one JSON object per momentum (matches dto.Momentum). The
// server never reads from the client — opening the connection is the
// only protocol input.
//
// Close codes used:
//   - 1000 normal: server shutdown or client disconnect.
//   - 1008 policy violation: auth failed (handled at upgrade time as 401).
//   - 1011 internal: dispatch failure or db error.
//   - 4000 slow_consumer: dispatch dropped frames; client must reconnect
//     with from_height of the last momentum it saw.
//
//nolint:contextcheck // WS connection lifecycle is detached from r.Context by design — see body comment
func MomentumsStream(signer *auth.Signer, hub *stream.Hub, repo streamMomentumsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Auth — header preferred, query-param fallback for browsers.
		tok, codeStr := streamToken(r)
		if tok == "" {
			httpx.WriteProblem(w, http.StatusUnauthorized, codeStr,
				"missing JWT (Authorization header or ?token= query)")
			return
		}
		if _, err := signer.Verify(tok); err != nil {
			httpx.WriteProblem(w, http.StatusUnauthorized, "invalid_token",
				"invalid token")
			return
		}

		// 2. Parse from_height if present.
		var fromHeight uint64
		if v := r.URL.Query().Get("from_height"); v != "" {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				httpx.WriteProblem(w, http.StatusBadRequest, "invalid_from_height",
					"from_height must be a non-negative integer")
				return
			}
			fromHeight = n
		}

		// 3. Upgrade. coder/websocket accepts any Origin by default,
		// which is what we want — auth is the trust boundary, not the
		// browser origin.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			// Accept already wrote the failure response on most paths.
			return
		}
		// Long-lived connection: detach from r.Context, which gets
		// canceled when net/http considers the handler returned. The
		// websocket library reads per-call deadlines from contexts we
		// pass into runLive / writeFrame.
		//nolint:contextcheck // WS connection outlives r.Context by design
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer func() { _ = conn.CloseNow() }()

		// 4. Subscribe BEFORE replay so we don't miss anything that
		// commits while the catch-up scan is running.
		sub := hub.Subscribe()
		defer sub.Close()

		if fromHeight > 0 {
			lastSent, err := replay(ctx, conn, repo, fromHeight)
			if err != nil {
				_ = conn.Close(websocket.StatusInternalError, "replay failed")
				return
			}
			// Any live frame whose height we already replayed gets
			// filtered in the live loop via lastSent.
			runLive(ctx, conn, sub, lastSent)
			return
		}
		runLive(ctx, conn, sub, 0)
	}
}

// streamToken extracts the JWT from either the Authorization header
// (preferred) or the ?token= query parameter (browser fallback —
// WebSocket clients in browsers cannot set custom headers on the
// upgrade request). Returns ("", code) on absence/malformation, where
// code distinguishes missing vs invalid for the error response.
func streamToken(r *http.Request) (token, code string) {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "Bearer "
		if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			return "", "invalid_token"
		}
		return strings.TrimSpace(h[len(prefix):]), ""
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t, ""
	}
	return "", "missing_token"
}

// replay pulls historical momentums from fromHeight up to the indexer's
// current tip, writing each as a WS frame. Returns the height of the
// last frame written so the live loop can skip duplicates that landed
// in the hub during the scan. Caps the scan at streamReplayMaxRows to
// bound the budget.
func replay(ctx context.Context, conn *websocket.Conn, repo streamMomentumsRepo, fromHeight uint64) (uint64, error) {
	latest, err := repo.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // empty table — nothing to replay
		}
		return 0, err
	}
	end := latest.Height
	if fromHeight > end {
		return fromHeight - 1, nil
	}
	if end-fromHeight+1 > streamReplayMaxRows {
		end = fromHeight + streamReplayMaxRows - 1
	}

	var lastSent uint64
	for h := fromHeight; h <= end; h++ {
		m, err := repo.GetByHeight(ctx, h)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Backfill gap — skip and keep going.
				continue
			}
			return lastSent, err
		}
		if err := writeFrame(ctx, conn, dto.FromMomentum(m)); err != nil {
			return lastSent, err
		}
		lastSent = h
	}
	return lastSent, nil
}

// runLive blocks on the subscriber channel + a ping ticker until the
// client goes away. lastSent suppresses any momentum height <= lastSent
// so a replay that overlapped with live arrivals doesn't double-emit.
func runLive(ctx context.Context, conn *websocket.Conn, sub *stream.Subscriber, lastSent uint64) {
	pingTicker := time.NewTicker(streamPingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "server shutdown")
			return

		case m, ok := <-sub.Recv():
			if !ok {
				_ = conn.Close(websocket.StatusInternalError, "hub closed")
				return
			}
			if sub.Lagged() > 0 {
				_ = conn.Close(4000, "slow_consumer: reconnect with from_height")
				return
			}
			if m.Height <= lastSent {
				continue
			}
			if err := writeFrame(ctx, conn, m); err != nil {
				return
			}

		case <-pingTicker.C:
			pingCtx, cancel := context.WithTimeout(ctx, streamWriteTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func writeFrame(ctx context.Context, conn *websocket.Conn, m *dto.Momentum) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, streamWriteTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, body)
}
