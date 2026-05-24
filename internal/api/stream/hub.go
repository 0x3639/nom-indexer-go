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

// defaultPerSubjectMax caps concurrent stream connections per JWT
// subject. Since the WS endpoint bypasses the per-request rate limiter
// (sliding windows are meaningless on a long-lived socket), this is the
// only knob preventing a single token from opening unbounded
// connections.
const defaultPerSubjectMax = 8

// Hub state transitions. Once stopped, the hub is terminal — a new Run
// goroutine on the same Hub is not supported (rebuild via New).
const (
	statePending int32 = iota
	stateRunning
	stateStopped
)

// Sentinel errors returned by Subscribe.
var (
	// ErrHubNotRunning is returned when the LISTEN loop has not started
	// or has exited. Handlers should map this to 503 Service Unavailable.
	ErrHubNotRunning = errors.New("stream hub not running")

	// ErrTooManyForSubject is returned when a JWT subject already has
	// PerSubjectMax open connections. Handlers should map this to 429
	// Too Many Requests.
	ErrTooManyForSubject = errors.New("too many concurrent streams for this subject")
)

// Subscriber is one consumer of the hub. The handler reads from Recv()
// and closes via Close() when the WebSocket goes away. The subject
// field is the JWT sub the connection was opened under; it's used for
// per-subject cap accounting on Close.
type Subscriber struct {
	ch      chan *dto.Momentum
	hub     *Hub
	subject string
	closed  atomic.Bool
	lagged  atomic.Uint64
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

// Close removes the subscriber from the hub and closes the channel.
// Safe to call multiple times; only the first call closes the channel,
// so drainAll on Run exit and an explicit handler Close cannot race
// into a double-close panic.
func (s *Subscriber) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.hub.unsubscribe(s)
	close(s.ch)
}

// Hub owns the single LISTEN connection and fans incoming momentums out
// to all current subscribers. Construct one per API process via New()
// and run it via Run().
type Hub struct {
	connectFn     func(context.Context) (*pgx.Conn, error)
	logger        *zap.Logger
	bufSize       int
	perSubjectMax int

	state atomic.Int32 // statePending / stateRunning / stateStopped

	mu         sync.RWMutex
	subs       map[*Subscriber]struct{}
	perSubject map[string]int
}

// Config bundles the inputs to New(). connectFn must produce a fresh
// *pgx.Conn (not from a pool) so the hub can hold it for the process
// lifetime without contending with request handlers.
type Config struct {
	ConnectFn func(context.Context) (*pgx.Conn, error)
	Logger    *zap.Logger
	// ClientBuf is the per-subscriber channel size (default 32).
	ClientBuf int
	// PerSubjectMax caps concurrent stream connections per JWT
	// subject (default 8). Zero or negative disables the cap.
	PerSubjectMax int
}

// New returns a Hub. It does NOT start the LISTEN loop — call Run().
func New(cfg Config) *Hub {
	buf := cfg.ClientBuf
	if buf <= 0 {
		buf = defaultClientBuffer
	}
	perSub := cfg.PerSubjectMax
	if perSub == 0 {
		perSub = defaultPerSubjectMax
	}
	return &Hub{
		connectFn:     cfg.ConnectFn,
		logger:        cfg.Logger,
		bufSize:       buf,
		perSubjectMax: perSub,
		subs:          make(map[*Subscriber]struct{}),
		perSubject:    make(map[string]int),
	}
}

// Running reports whether the hub is currently in its LISTEN loop.
// Handlers should check this before subscribing — there's also a state
// re-check inside Subscribe under lock, but the fast path avoids the
// mutex when the hub is clearly down.
func (h *Hub) Running() bool {
	return h.state.Load() == stateRunning
}

// Subscribe registers a new client. subject is the JWT sub claim from
// the request, used for per-subject connection-cap accounting.
//
// Returns ErrHubNotRunning if Run hasn't started or has already exited;
// returns ErrTooManyForSubject if the subject is at its cap. Always
// call Subscriber.Close() to release the slot.
func (h *Hub) Subscribe(subject string) (*Subscriber, error) {
	if h.state.Load() != stateRunning {
		return nil, ErrHubNotRunning
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	// Re-check under lock so a concurrent drainAll can't slip a
	// subscriber in after state flipped to stopped.
	if h.state.Load() != stateRunning {
		return nil, ErrHubNotRunning
	}
	if h.perSubjectMax > 0 && h.perSubject[subject] >= h.perSubjectMax {
		return nil, ErrTooManyForSubject
	}
	s := &Subscriber{
		ch:      make(chan *dto.Momentum, h.bufSize),
		hub:     h,
		subject: subject,
	}
	h.subs[s] = struct{}{}
	h.perSubject[subject]++
	return s, nil
}

// SubscriberCount returns the current live subscriber count. Intended
// for /metrics exporters.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// SubjectCount returns the number of open connections currently
// attributed to subject — useful for the handler to surface the cap in
// 429 problem-detail bodies.
func (h *Hub) SubjectCount(subject string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.perSubject[subject]
}

func (h *Hub) unsubscribe(s *Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[s]; !ok {
		// Already removed by drainAll. The channel-close in
		// Subscriber.Close handles cleanup; nothing more to do.
		return
	}
	delete(h.subs, s)
	h.perSubject[s.subject]--
	if h.perSubject[s.subject] <= 0 {
		delete(h.perSubject, s.subject)
	}
}

// drainAll flips state to stopped and closes every current subscriber.
// Called when Run returns (clean shutdown or unrecoverable failure) so
// existing WS handlers unblock their <-sub.Recv() loops promptly
// instead of waiting forever on a dead hub.
func (h *Hub) drainAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Store(stateStopped)
	for s := range h.subs {
		if !s.closed.Swap(true) {
			close(s.ch)
		}
	}
	h.subs = make(map[*Subscriber]struct{})
	h.perSubject = make(map[string]int)
}

// Run owns a single dedicated pgx.Conn, issues LISTEN momentum_new, and
// loops on WaitForNotification until ctx is canceled. Returns nil on
// clean shutdown, error on unrecoverable connection failure. Callers
// should run this in a goroutine and treat a returned error as fatal
// for the streaming subsystem (the API itself keeps serving REST).
//
// On exit (clean or error), all existing subscribers are closed and
// the hub state flips to stopped so new Subscribe calls return
// ErrHubNotRunning. The hub is not restartable — rebuild via New.
//
//nolint:contextcheck // shutdown-close uses a detached ctx by design
func (h *Hub) Run(ctx context.Context) error {
	defer h.drainAll()

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
	h.state.Store(stateRunning)
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

// MarkRunningForTest puts the hub into the running state without
// actually running the LISTEN loop, so handler-level tests can
// Subscribe without spinning up Postgres.
func MarkRunningForTest(h *Hub) {
	h.state.Store(stateRunning)
}

// MarkStoppedForTest drains all subscribers and flips state to stopped
// — used by tests that want to observe Subscribe rejecting after Run
// has exited.
func MarkStoppedForTest(h *Hub) {
	h.drainAll()
}
