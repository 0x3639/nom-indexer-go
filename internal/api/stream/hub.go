package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
)

// Channel name the indexer NOTIFY uses. Kept in sync with
// internal/indexer/processor.go::notifyMomentum.
const channelName = "momentum_new"

// defaultClientBuffer is the per-subscriber channel size. Sized at 32 so
// a brief network blip (a few seconds of unflushed frames) doesn't
// immediately mark the client as lagged on a 10s block time.
const defaultClientBuffer = 32

// Subscriber is one consumer of the hub. The handler reads from Recv()
// and closes via Close() when the WebSocket goes away.
type Subscriber struct {
	ch     chan *dto.Momentum
	hub    *Hub
	closed atomic.Bool
	lagged atomic.Uint64
}

// Recv returns the channel the handler reads new momentums from.
func (s *Subscriber) Recv() <-chan *dto.Momentum {
	return s.ch
}

// Lagged returns the number of momentums dropped because the
// subscriber's channel was full. Non-zero means the handler is too
// slow to keep up and should close the connection.
func (s *Subscriber) Lagged() uint64 {
	return s.lagged.Load()
}

// Close removes the subscriber from the hub and drains the channel.
// Safe to call multiple times.
func (s *Subscriber) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.hub.unsubscribe(s)
}

// Hub owns the single LISTEN connection and fans incoming momentums out
// to all current subscribers. Construct one per API process via New()
// and run it via Run().
type Hub struct {
	connectFn func(context.Context) (*pgx.Conn, error)
	logger    *zap.Logger
	bufSize   int

	mu   sync.RWMutex
	subs map[*Subscriber]struct{}
}

// Config bundles the inputs to New(). connectFn must produce a fresh
// *pgx.Conn (not from a pool) so the hub can hold it for the process
// lifetime without contending with request handlers.
type Config struct {
	ConnectFn func(context.Context) (*pgx.Conn, error)
	Logger    *zap.Logger
	ClientBuf int
}

// New returns a Hub. It does NOT start the LISTEN loop — call Run().
func New(cfg Config) *Hub {
	buf := cfg.ClientBuf
	if buf <= 0 {
		buf = defaultClientBuffer
	}
	return &Hub{
		connectFn: cfg.ConnectFn,
		logger:    cfg.Logger,
		bufSize:   buf,
		subs:      make(map[*Subscriber]struct{}),
	}
}

// Subscribe registers a new client. The returned Subscriber starts
// receiving momentums as soon as the next NOTIFY lands. Always call
// Close() to release the slot.
func (h *Hub) Subscribe() *Subscriber {
	s := &Subscriber{
		ch:  make(chan *dto.Momentum, h.bufSize),
		hub: h,
	}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// SubscriberCount returns the current live subscriber count. Intended
// for /metrics exporters.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

func (h *Hub) unsubscribe(s *Subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
	// Closing the channel after delete is safe: no future dispatch can
	// reach s, and any in-flight dispatch sees the bounded select drop
	// the write rather than panic on a closed receive.
	close(s.ch)
}

// Run owns a single dedicated pgx.Conn, issues LISTEN momentum_new, and
// loops on WaitForNotification until ctx is canceled. Returns nil on
// clean shutdown, error on unrecoverable connection failure. Callers
// should run this in a goroutine and treat a returned error as fatal
// for the streaming subsystem (the API itself keeps serving REST).
//
//nolint:contextcheck // shutdown-close uses a detached ctx by design
func (h *Hub) Run(ctx context.Context) error {
	conn, err := h.connectFn(ctx)
	if err != nil {
		return fmt.Errorf("stream hub connect: %w", err)
	}
	defer func() {
		// Best-effort close on shutdown. Detached context with a short
		// deadline because the outer ctx is by definition canceled
		// here (that's what triggered Run to return).
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx) //nolint:contextcheck // shutdown path; outer ctx is canceled
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+channelName); err != nil {
		return fmt.Errorf("stream hub LISTEN: %w", err)
	}
	h.logger.Info("stream hub listening", zap.String("channel", channelName))

	for {
		notif, err := conn.WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				h.logger.Info("stream hub shutting down")
				return nil
			}
			return fmt.Errorf("stream hub wait: %w", err)
		}
		var m dto.Momentum
		if err := json.Unmarshal([]byte(notif.Payload), &m); err != nil {
			// Malformed payload from the indexer is a bug, not a runtime
			// failure — log and continue so a single bad notify doesn't
			// kill the stream.
			h.logger.Warn("stream hub malformed notify payload",
				zap.String("payload", notif.Payload), zap.Error(err))
			continue
		}
		h.dispatch(&m)
	}
}

// dispatch writes m to every current subscriber non-blockingly. A
// subscriber whose channel is full is marked lagged but kept in the
// set; the handler observes Lagged() and closes the connection on its
// own schedule. Holding the read lock for the whole loop is fine —
// Subscribe/unsubscribe take the write lock so they wait until
// dispatch finishes the current momentum.
func (h *Hub) dispatch(m *dto.Momentum) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		select {
		case s.ch <- m:
		default:
			s.lagged.Add(1)
		}
	}
}

// DispatchForTest synthesizes a notification without going through
// Postgres. Intended for handler-level unit tests that need to inject a
// momentum into the fan-out without standing up a real NOTIFY pipeline.
// Production callers should never use this — momentums must come from
// the LISTEN loop so the indexer remains the single source of truth.
func DispatchForTest(h *Hub, m *dto.Momentum) {
	h.dispatch(m)
}
