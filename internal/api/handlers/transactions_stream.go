package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

// streamTxRepo is the slice of AccountBlockRepository the transactions
// stream handler needs for replay. Interface so handler tests can swap
// in a fake without Postgres.
type streamTxRepo interface {
	ListByMomentumHeightRange(
		ctx context.Context,
		fromMomentumHeight, toMomentumHeight int64,
		addressFilter string,
		limit int,
	) ([]*models.AccountBlock, error)
}

// streamLatestMomentumRepo is the subset of MomentumRepository the
// transactions handler needs to bound a from_height replay window
// against the chain tip. Reusing the existing interface from the
// momentums handler would force a circular re-export; smaller interface
// here is cleaner.
type streamLatestMomentumRepo interface {
	GetLatest(ctx context.Context) (*models.Momentum, error)
}

// TransactionsStream handles GET /api/v1/transactions/stream as a WebSocket.
//
// Auth (same as momentums/stream): Bearer header OR ?token= query
// param. Browsers can only use the query-param form.
//
// Query parameters:
//   - address (optional): when set, server filters frames to blocks
//     where address matches sender (address) OR recipient
//     (to_address). Without this, the stream is the full firehose of
//     every account_block the indexer commits.
//   - from_height (optional, int64): the MOMENTUM height at which to
//     start the replay. Catches up via a single range scan from that
//     momentum_height up to the chain tip (capped at streamReplayMaxRows)
//     before switching to live.
//
// Frames: one JSON object per account_block (matches dto.AccountBlock).
// Close codes mirror the momentums stream: 1000 normal, 1011 internal,
// 4000 slow_consumer.
//
//nolint:contextcheck // WS connection lifecycle is detached from r.Context by design — see body comment
func TransactionsStream(
	signer *auth.Signer,
	hub *stream.Hub[*dto.AccountBlock],
	txRepo streamTxRepo,
	momentumRepo streamLatestMomentumRepo,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Auth — header preferred, query-param fallback for browsers.
		tok, codeStr := streamToken(r)
		if tok == "" {
			httpx.WriteProblem(w, http.StatusUnauthorized, codeStr,
				"missing JWT (Authorization header or ?token= query)")
			return
		}
		claims, err := signer.Verify(tok)
		if err != nil {
			httpx.WriteProblem(w, http.StatusUnauthorized, "invalid_token",
				"invalid token")
			return
		}
		subject := claims.Subject

		// 2. Parse optional query params.
		var fromMomentumHeight int64
		if v := r.URL.Query().Get("from_height"); v != "" {
			n, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil || n < 0 {
				httpx.WriteProblem(w, http.StatusBadRequest, "invalid_from_height",
					"from_height must be a non-negative integer")
				return
			}
			fromMomentumHeight = n
		}
		addressFilter := r.URL.Query().Get("address")

		// 3. Subscribe BEFORE upgrade so failures surface as plain HTTP.
		sub, err := hub.Subscribe(subject)
		if err != nil {
			switch {
			case errors.Is(err, stream.ErrHubNotRunning):
				httpx.WriteProblem(w, http.StatusServiceUnavailable,
					"stream_unavailable",
					"transactions stream is temporarily unavailable")
			case errors.Is(err, stream.ErrTooManyForSubject):
				httpx.WriteProblem(w, http.StatusTooManyRequests,
					"stream_subject_limit",
					"too many concurrent streams for this token's subject")
			default:
				httpx.WriteProblem(w, http.StatusInternalServerError,
					"internal_error", "stream subscribe failed")
			}
			return
		}
		defer sub.Close()

		// 4. Upgrade.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx := conn.CloseRead(context.Background())

		// 5. Optional replay + filter + live.
		var lastSent int64
		if fromMomentumHeight > 0 {
			h, err := replayTransactions(ctx, conn, txRepo, momentumRepo, fromMomentumHeight, addressFilter)
			if err != nil {
				_ = conn.Close(websocket.StatusInternalError, "replay failed")
				return
			}
			lastSent = h
		}
		runTxLive(ctx, conn, sub, addressFilter, lastSent)
	}
}

// replayTransactions issues a single range scan against account_blocks
// from fromMomentumHeight up to the indexer's tip (capped). Writes
// each row as a WS frame. Returns the highest momentum_height written
// so the live loop can suppress frames at-or-below that height in case
// the hub buffered new commits mid-scan.
func replayTransactions(
	ctx context.Context,
	conn *websocket.Conn,
	txRepo streamTxRepo,
	momentumRepo streamLatestMomentumRepo,
	fromMomentumHeight int64,
	addressFilter string,
) (int64, error) {
	latest, err := momentumRepo.GetLatest(ctx)
	if err != nil {
		// Empty table or unreachable: no replay, fall through to live.
		// A fresh chain or transient blip shouldn't abort the upgrade.
		return fromMomentumHeight - 1, nil //nolint:nilerr // graceful degradation
	}
	end := int64(latest.Height)
	if fromMomentumHeight > end {
		return fromMomentumHeight - 1, nil
	}
	if end-fromMomentumHeight+1 > streamReplayMaxRows {
		end = fromMomentumHeight + streamReplayMaxRows - 1
	}

	rows, err := txRepo.ListByMomentumHeightRange(ctx, fromMomentumHeight, end, addressFilter, streamReplayMaxRows)
	if err != nil {
		return 0, err
	}
	var lastSent int64
	for _, ab := range rows {
		if err := writeTxFrame(ctx, conn, dto.FromAccountBlock(ab)); err != nil {
			return lastSent, err
		}
		if ab.MomentumHeight > lastSent {
			lastSent = ab.MomentumHeight
		}
	}
	return lastSent, nil
}

// runTxLive blocks on the subscriber channel + ping ticker, applying
// the optional per-address filter before each WS write. lastSent
// suppresses any frame whose momentum_height <= lastSent so a replay
// that overlapped with live arrivals doesn't double-emit.
func runTxLive(
	ctx context.Context,
	conn *websocket.Conn,
	sub *stream.Subscriber[*dto.AccountBlock],
	addressFilter string,
	lastSent int64,
) {
	pingTicker := time.NewTicker(streamPingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "server shutdown")
			return

		case ab, ok := <-sub.Recv():
			if !ok {
				_ = conn.Close(websocket.StatusInternalError, "hub closed")
				return
			}
			if sub.Lagged() > 0 {
				_ = conn.Close(4000, "slow_consumer: reconnect with from_height")
				return
			}
			if ab.MomentumHeight <= lastSent {
				continue
			}
			if addressFilter != "" && ab.Address != addressFilter && ab.ToAddress != addressFilter {
				continue
			}
			if err := writeTxFrame(ctx, conn, ab); err != nil {
				return
			}
			if ab.MomentumHeight > lastSent {
				lastSent = ab.MomentumHeight
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

func writeTxFrame(ctx context.Context, conn *websocket.Conn, ab *dto.AccountBlock) error {
	body, err := json.Marshal(ab)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, streamWriteTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, body)
}
