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

// Reconnect backoff bounds for the LISTEN loop. A transient DB blip
// (failover, network glitch) should self-heal without a process
// restart; cap matches typical k8s service-mesh retry budgets.
const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// Hub state transitions:
//
//	pending → running   (LISTEN succeeded; Subscribe accepts)
//	running → degraded  (notification error; Subscribe rejects, will retry)
//	degraded → running  (reconnect succeeded)
//	any → stopped       (ctx canceled; Run exiting permanently)
//
// Subscribe accepts only in state running. Degraded and stopped both
// reject with ErrHubNotRunning — callers don't need to distinguish
// "transient down" from "terminal down"; reconnect with backoff is the
// same client-side strategy either way.
const (
	statePending int32 = iota
	stateRunning
	stateDegraded
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
	// subject. The Go zero value (field omitted) applies the default
	// (8); a negative value disables the cap entirely. Setting an
	// explicit zero is treated the same as omitting the field.
	PerSubjectMax int
}

// New returns a Hub. It does NOT start the LISTEN loop — call Run().
func New(cfg Config) *Hub {
	buf := cfg.ClientBuf
	if buf <= 0 {
		buf = defaultClientBuffer
	}
	// PerSubjectMax: 0 (the Go zero value, ie. field omitted) →
	// default; negative → disabled. See Config.PerSubjectMax doc.
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

// closeAllSubs closes every current subscriber's channel and clears
// the maps WITHOUT changing hub state. Called both between reconnect
// attempts (so existing WS clients drop their stalled connections and
// reconnect once the hub is healthy) and on terminal shutdown (via
// drainAll). After this, every Subscriber the caller previously held
// will see a closed channel.
func (h *Hub) closeAllSubs() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if !s.closed.Swap(true) {
			close(s.ch)
		}
	}
	h.subs = make(map[*Subscriber]struct{})
	h.perSubject = make(map[string]int)
}

// drainAll flips state to stopped (terminal) and closes every current
// subscriber. Called when Run is about to return permanently — ctx
// canceled — so callers' <-sub.Recv() loops unblock promptly and new
// Subscribe calls fail with ErrHubNotRunning.
func (h *Hub) drainAll() {
	h.state.Store(stateStopped)
	h.closeAllSubs()
}

// sleep waits for d or ctx cancellation. Returns true if the duration
// elapsed normally, false if ctx was canceled (caller should exit).
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func nextBackoff(d time.Duration) time.Duration {
	next := d * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// Run owns the LISTEN lifecycle for the process. The outer loop wraps
// connect + LISTEN + WaitForNotification in a reconnect-with-backoff
// retry so a transient DB blip or LISTEN connection drop self-heals
// without a process restart. Returns nil only when ctx is canceled
// (clean shutdown); any other failure path is treated as transient
// and retried up to maxBackoff cadence.
//
// Subscriber semantics across reconnects:
//
//   - Active subscribers are closed every time the underlying
//     connection drops (closeAllSubs). WS handlers observe the
//     channel close and tear down their client connections; clients
//     reconnect with from_height of their last seen momentum to fill
//     the gap.
//   - During reconnect attempts the hub sits in stateDegraded and
//     Subscribe rejects with ErrHubNotRunning. Clients retry until
//     the hub flips back to stateRunning (typically within seconds).
//   - The transition from stateRunning to stateDegraded happens
//     INSIDE connectAndListen (before closeAllSubs), so a Subscribe
//     racing with a connection drop cannot land on a hub that is
//     simultaneously closing its subscribers. See comment there.
//
// Backoff is reset to initialBackoff whenever connectAndListen
// reports that it successfully reached LISTEN at any point during
// the iteration — so a long-lived hub that drops once after hours
// of healthy operation reconnects in 1 s, not at the prior
// maxBackoff ceiling.
//
//nolint:contextcheck // detached ctx used only on shutdown / pgx close
func (h *Hub) Run(ctx context.Context) error {
	defer h.drainAll()
	backoff := initialBackoff

	for {
		listened, err := h.connectAndListen(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			// connectAndListen returns nil only when ctx is canceled
			// (handled above). Defensive — should not hit.
			return nil
		}

		if listened {
			// A session lived past LISTEN at least once — the prior
			// backoff was earned by failures that may not recur. Reset.
			backoff = initialBackoff
		}
		h.logger.Warn("stream hub connect/listen failed; will retry",
			zap.Error(err), zap.Duration("backoff", backoff))
		if !sleep(ctx, backoff) {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
}

// connectAndListen runs one connect→LISTEN→waitLoop cycle. Returns
// (listened=false, err) on a connect/LISTEN failure, (listened=true,
// err) on a wait-loop failure after a session was successfully
// established, or (listened=true, nil) on a clean ctx-canceled exit.
// The listened bool tells Run whether to reset its backoff counter.
//
// State transition order on a notification-wait failure (critical to
// avoid a Subscribe race onto a dead hub):
//
//  1. flip state to stateDegraded (Subscribe now rejects)
//  2. close all current subscribers (handlers exit promptly)
//  3. close the pgx connection
//
// Doing 1 BEFORE 2 means a Subscribe racing with the connection drop
// either lands while state is still Running (and gets closed by step
// 2 — handler reconnects with from_height) OR after state flipped to
// Degraded (and is rejected with 503 immediately). The previous code
// had a window where state was Running, subs were already closed,
// and a new Subscribe would land in an empty map awaiting frames
// that would never come.
//
//nolint:contextcheck // pgx.Close on a detached ctx is the standard cleanup pattern
func (h *Hub) connectAndListen(ctx context.Context) (listened bool, err error) {
	conn, err := h.connectFn(ctx)
	if err != nil {
		return false, fmt.Errorf("connect: %w", err)
	}
	closeConn := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}

	if _, err := conn.Exec(ctx, "LISTEN "+channelName); err != nil {
		closeConn()
		return false, fmt.Errorf("LISTEN: %w", err)
	}
	h.state.Store(stateRunning)
	h.logger.Info("stream hub listening", zap.String("channel", channelName))

	for {
		notif, werr := conn.WaitForNotification(ctx)
		if werr != nil {
			// Order matters — see method comment. State flips first so
			// a racing Subscribe sees Degraded immediately; subs are
			// then closed; conn is closed last.
			if ctx.Err() == nil {
				h.state.Store(stateDegraded)
			}
			h.closeAllSubs()
			closeConn()
			if errors.Is(werr, context.Canceled) || errors.Is(werr, context.DeadlineExceeded) {
				h.logger.Info("stream hub shutting down")
				return true, nil
			}
			return true, fmt.Errorf("wait: %w", werr)
		}
		var m dto.Momentum
		if uerr := json.Unmarshal([]byte(notif.Payload), &m); uerr != nil {
			// Malformed payload from the indexer is a bug, not a runtime
			// failure — log and continue so a single bad notify doesn't
			// kill the stream.
			h.logger.Warn("stream hub malformed notify payload",
				zap.String("payload", notif.Payload), zap.Error(uerr))
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
