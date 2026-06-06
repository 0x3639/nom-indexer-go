package webhooks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDispatcher_DeliversAndSigns(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies [][]byte
		sigs   []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, buf)
		sigs = append(sigs, r.Header.Get("X-Webhook-Signature"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := New([]Endpoint{{URL: srv.URL, Secret: "s3cr3t"}}, 2*time.Second, 1, nil)
	d.Start()
	defer d.Stop()

	d.Emit(Event{Type: "momentum.inserted", Payload: map[string]any{"height": "42"}})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(bodies)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(bodies))
	}
	var got Event
	if err := json.Unmarshal(bodies[0], &got); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if got.Type != "momentum.inserted" {
		t.Errorf("type = %q", got.Type)
	}
	if want := ComputeSignature("s3cr3t", bodies[0]); sigs[0] != want {
		t.Errorf("signature = %q, want %q", sigs[0], want)
	}
}

func TestDispatcher_EventFilter(t *testing.T) {
	d := New([]Endpoint{{URL: "http://x", Events: []string{"account_block.inserted"}}}, time.Second, 1, nil)
	if d.wants(d.endpoints[0], "momentum.inserted") {
		t.Error("should not want unsubscribed event")
	}
	if !d.wants(d.endpoints[0], "account_block.inserted") {
		t.Error("should want subscribed event")
	}
}

func TestDispatcher_EmitAfterStopIsSafe(t *testing.T) {
	d := New([]Endpoint{{URL: "http://127.0.0.1:0"}}, time.Second, 0, nil)
	d.Start()
	d.Stop()
	// Must not panic.
	d.Emit(Event{Type: "momentum.inserted", Payload: map[string]any{"h": "1"}})
	// Double stop must not panic.
	d.Stop()
}

func TestDispatcher_ConcurrentEmitAndStop(t *testing.T) {
	d := New([]Endpoint{{URL: "http://127.0.0.1:0"}}, time.Second, 0, nil)
	d.Start()

	const emitters = 8
	var wg sync.WaitGroup
	wg.Add(emitters)
	for i := 0; i < emitters; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				// Some of these land before Stop, some after; all must be
				// panic-free (the queue is never closed).
				d.Emit(Event{Type: "momentum.inserted", Payload: map[string]any{"j": j}})
			}
		}()
	}

	// Race Stop against the emitters.
	go d.Stop()

	wg.Wait()
	// Ensure shutdown completed; second Stop is a no-op wait.
	d.Stop()
}
