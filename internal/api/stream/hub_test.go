package stream

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
)

func newTestHub() *Hub {
	return New(Config{Logger: zap.NewNop(), ClientBuf: 4})
}

func TestHub_SubscribeAndDispatch(t *testing.T) {
	h := newTestHub()
	s := h.Subscribe()
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
		subs[i] = h.Subscribe()
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
	slow := h.Subscribe()
	defer slow.Close()

	// Fill the buffer + drop 6 more — slow never reads.
	for i := uint64(0); i < 10; i++ {
		h.dispatch(&dto.Momentum{Height: i})
	}

	if got := slow.Lagged(); got != 6 {
		t.Errorf("lagged = %d, want 6 (10 dispatched, buffer=4)", got)
	}
	// Now drain the buffer and confirm we got the first 4 in order.
	for i := 0; i < 4; i++ {
		m := <-slow.Recv()
		if m.Height != uint64(i) {
			t.Errorf("position %d: got height %d, want %d", i, m.Height, i)
		}
	}
}

func TestHub_FastSubscriberNotPenalizedBySlowOne(t *testing.T) {
	h := newTestHub()
	slow := h.Subscribe()
	defer slow.Close()
	fast := h.Subscribe()
	defer fast.Close()

	// Drain fast in a goroutine.
	var received atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range fast.Recv() {
			received.Add(1)
		}
	}()

	// Dispatch with a tiny pause so the goroutine has a chance to drain
	// between sends — the goal of this test is to prove the dispatch
	// loop doesn't block the fast subscriber on the slow one, NOT to
	// stress the buffer (TestHub_SlowSubscriberLagsRatherThanBlocks
	// covers that case).
	const n = 50
	for i := uint64(0); i < n; i++ {
		h.dispatch(&dto.Momentum{Height: i})
		time.Sleep(200 * time.Microsecond)
	}

	// Wait for the goroutine to drain everything dispatched.
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
	// slow lagged because nobody drained it — proof the dispatch loop
	// did not block waiting for slow.
	if slow.Lagged() == 0 {
		t.Error("slow subscriber should have lagged but Lagged() = 0")
	}
}

func TestHub_CloseRemovesFromSet(t *testing.T) {
	h := newTestHub()
	s := h.Subscribe()
	if h.SubscriberCount() != 1 {
		t.Fatalf("count = %d, want 1", h.SubscriberCount())
	}
	s.Close()
	if h.SubscriberCount() != 0 {
		t.Errorf("count after Close = %d, want 0", h.SubscriberCount())
	}
	// Channel is closed — recv yields zero value with ok=false.
	if _, ok := <-s.Recv(); ok {
		t.Error("expected closed channel after Close()")
	}
	// Double-close must not panic.
	s.Close()
}

func TestHub_DispatchWithNoSubscribersIsHarmless(t *testing.T) {
	h := newTestHub()
	// No subscribers — dispatch should be a no-op, no panic.
	h.dispatch(&dto.Momentum{Height: 1})
}
