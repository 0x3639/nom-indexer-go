package stream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
)

func newTestHub() *Hub {
	h := New(Config{Logger: zap.NewNop(), ClientBuf: 4, PerSubjectMax: 100})
	MarkRunningForTest(h)
	return h
}

func mustSubscribe(t *testing.T, h *Hub, subject string) *Subscriber {
	t.Helper()
	s, err := h.Subscribe(subject)
	if err != nil {
		t.Fatalf("Subscribe(%q): %v", subject, err)
	}
	return s
}

func TestHub_SubscribeAndDispatch(t *testing.T) {
	h := newTestHub()
	s := mustSubscribe(t, h, "alice")
	defer s.Close()

	want := &dto.Momentum{Height: 42, Hash: "abc"}
	h.dispatch(want)

	select {
	case got := <-s.Recv():
		if got.Height != 42 || got.Hash != "abc" {
			t.Errorf("got %+v, want height=42 hash=abc", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber did not receive momentum")
	}
}

func TestHub_FanOutAcrossSubscribers(t *testing.T) {
	h := newTestHub()
	subs := make([]*Subscriber, 5)
	for i := range subs {
		subs[i] = mustSubscribe(t, h, "alice")
		defer subs[i].Close()
	}

	want := &dto.Momentum{Height: 7}
	h.dispatch(want)

	for i, s := range subs {
		select {
		case got := <-s.Recv():
			if got.Height != 7 {
				t.Errorf("sub %d: got height %d, want 7", i, got.Height)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("sub %d: no momentum received", i)
		}
	}
}

func TestHub_SlowSubscriberLagsRatherThanBlocks(t *testing.T) {
	h := newTestHub() // ClientBuf=4
	slow := mustSubscribe(t, h, "alice")
	defer slow.Close()

	// Fill the buffer + drop 6 more — slow never reads.
	for i := uint64(0); i < 10; i++ {
		h.dispatch(&dto.Momentum{Height: i})
	}

	if got := slow.Lagged(); got != 6 {
		t.Errorf("lagged = %d, want 6 (10 dispatched, buffer=4)", got)
	}
	for i := 0; i < 4; i++ {
		m := <-slow.Recv()
		if m.Height != uint64(i) {
			t.Errorf("position %d: got height %d, want %d", i, m.Height, i)
		}
	}
}

func TestHub_FastSubscriberNotPenalizedBySlowOne(t *testing.T) {
	h := newTestHub()
	slow := mustSubscribe(t, h, "slow-sub")
	defer slow.Close()
	fast := mustSubscribe(t, h, "fast-sub")
	defer fast.Close()

	var received atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range fast.Recv() {
			received.Add(1)
		}
	}()

	const n = 50
	for i := uint64(0); i < n; i++ {
		h.dispatch(&dto.Momentum{Height: i})
		time.Sleep(200 * time.Microsecond)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for received.Load() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	fast.Close()
	wg.Wait()

	if got := received.Load(); got != n {
		t.Errorf("fast subscriber got %d, want %d", got, n)
	}
	if fast.Lagged() != 0 {
		t.Errorf("fast subscriber Lagged() = %d, want 0 (slow client should not block fast)", fast.Lagged())
	}
	if slow.Lagged() == 0 {
		t.Error("slow subscriber should have lagged but Lagged() = 0")
	}
}

func TestHub_CloseRemovesFromSet(t *testing.T) {
	h := newTestHub()
	s := mustSubscribe(t, h, "alice")
	if h.SubscriberCount() != 1 {
		t.Fatalf("count = %d, want 1", h.SubscriberCount())
	}
	if h.SubjectCount("alice") != 1 {
		t.Errorf("SubjectCount(alice) = %d, want 1", h.SubjectCount("alice"))
	}
	s.Close()
	if h.SubscriberCount() != 0 {
		t.Errorf("count after Close = %d, want 0", h.SubscriberCount())
	}
	if h.SubjectCount("alice") != 0 {
		t.Errorf("SubjectCount(alice) after Close = %d, want 0", h.SubjectCount("alice"))
	}
	if _, ok := <-s.Recv(); ok {
		t.Error("expected closed channel after Close()")
	}
	// Double-close must not panic or affect counters.
	s.Close()
}

func TestHub_DispatchWithNoSubscribersIsHarmless(t *testing.T) {
	h := newTestHub()
	h.dispatch(&dto.Momentum{Height: 1})
}

func TestHub_PerSubjectCap(t *testing.T) {
	h := New(Config{Logger: zap.NewNop(), ClientBuf: 4, PerSubjectMax: 3})
	MarkRunningForTest(h)

	subs := make([]*Subscriber, 3)
	for i := range subs {
		s, err := h.Subscribe("alice")
		if err != nil {
			t.Fatalf("Subscribe alice #%d: %v", i, err)
		}
		subs[i] = s
	}
	defer func() {
		for _, s := range subs {
			s.Close()
		}
	}()

	// 4th alice connection should fail.
	if _, err := h.Subscribe("alice"); !errors.Is(err, ErrTooManyForSubject) {
		t.Errorf("expected ErrTooManyForSubject; got %v", err)
	}

	// bob is a different subject — fine.
	bob, err := h.Subscribe("bob")
	if err != nil {
		t.Fatalf("Subscribe bob: %v", err)
	}
	defer bob.Close()

	// After closing one alice, a new alice should succeed.
	subs[0].Close()
	alice4, err := h.Subscribe("alice")
	if err != nil {
		t.Fatalf("Subscribe alice after Close: %v", err)
	}
	defer alice4.Close()
}

func TestHub_StatePending_RejectsSubscribe(t *testing.T) {
	h := New(Config{Logger: zap.NewNop()})
	// Don't call MarkRunningForTest — hub is pending.
	if _, err := h.Subscribe("alice"); !errors.Is(err, ErrHubNotRunning) {
		t.Errorf("Subscribe on pending hub: got %v, want ErrHubNotRunning", err)
	}
	if h.Running() {
		t.Error("Running() = true on a pending hub")
	}
}

// TestHub_RunReconnectsOnConnectFailure forces the connectFn to fail
// every call. The hub should retry with backoff rather than giving up
// after the first failure (which was the pre-fix behavior — Run
// returned permanently on any connect error).
//
// We let Run iterate for ~3 seconds then cancel; the initial backoff
// is 1s (initialBackoff const), so we expect 3+ connectFn calls.
func TestHub_RunReconnectsOnConnectFailure(t *testing.T) {
	var calls atomic.Int32
	connectErr := errors.New("simulated DB outage")
	h := New(Config{
		Logger:    zap.NewNop(),
		ClientBuf: 4,
		ConnectFn: func(_ context.Context) (*pgx.Conn, error) {
			calls.Add(1)
			return nil, connectErr
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	runErr := make(chan error, 1)
	go func() { runErr <- h.Run(ctx) }()

	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run returned %v, want nil on ctx-canceled exit", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Run did not exit within timeout")
	}

	if got := calls.Load(); got < 2 {
		t.Errorf("connectFn called %d times, want >= 2 (proves retry loop)", got)
	}
	if h.Running() {
		t.Error("Running() = true after ctx canceled — should be stopped")
	}
}

// TestHub_CloseAllSubsKeepsStateRunning verifies the split between
// closeAllSubs (for reconnect — closes channels, leaves state alone)
// and drainAll (for terminal shutdown — both). The reconnect path
// closes existing subs so client handlers exit; new clients can then
// subscribe and start receiving frames once the connection is back.
func TestHub_CloseAllSubsKeepsStateRunning(t *testing.T) {
	h := newTestHub()
	a := mustSubscribe(t, h, "alice")
	b := mustSubscribe(t, h, "bob")

	h.closeAllSubs()

	for i, s := range []*Subscriber{a, b} {
		select {
		case _, ok := <-s.Recv():
			if ok {
				t.Errorf("sub %d: expected closed channel, got value", i)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("sub %d: Recv() did not unblock after closeAllSubs", i)
		}
	}

	if !h.Running() {
		t.Error("closeAllSubs flipped state; should only happen on drainAll")
	}
	c, err := h.Subscribe("carol")
	if err != nil {
		t.Errorf("Subscribe after closeAllSubs: %v", err)
	} else {
		c.Close()
	}
}

func TestHub_StateStopped_DrainsExistingAndRejectsNew(t *testing.T) {
	h := newTestHub()
	a := mustSubscribe(t, h, "alice")
	b := mustSubscribe(t, h, "alice")
	defer a.Close()
	defer b.Close()

	MarkStoppedForTest(h) // simulates Run exit

	// Both existing subscribers should see their channels closed.
	for i, s := range []*Subscriber{a, b} {
		select {
		case _, ok := <-s.Recv():
			if ok {
				t.Errorf("sub %d: expected closed channel, got value", i)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("sub %d: Recv() did not unblock after hub stopped", i)
		}
	}

	// New subscribes are rejected.
	if _, err := h.Subscribe("alice"); !errors.Is(err, ErrHubNotRunning) {
		t.Errorf("Subscribe after stop: got %v, want ErrHubNotRunning", err)
	}
	if h.Running() {
		t.Error("Running() = true after MarkStoppedForTest")
	}
}
