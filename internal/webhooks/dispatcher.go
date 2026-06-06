// Package webhooks delivers fire-and-forget event notifications to configured
// HTTP endpoints. Delivery is best-effort: failures are retried a bounded
// number of times and then dropped (with a log line). It never blocks the
// indexer's sync loop.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Event is one notification.
type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// Endpoint is a subscriber.
type Endpoint struct {
	URL    string
	Secret string
	Events []string // empty = all
}

// Dispatcher fans events out to endpoints via a buffered queue + worker.
type Dispatcher struct {
	endpoints  []Endpoint
	timeout    time.Duration
	maxRetries int
	logger     *zap.Logger
	queue      chan Event
	client     *http.Client
	done       chan struct{}
	quit       chan struct{}
	closed     atomic.Bool
	startOnce  sync.Once
	stopOnce   sync.Once
}

// New builds a Dispatcher. logger may be nil (a no-op logger is used).
func New(endpoints []Endpoint, timeout time.Duration, maxRetries int, logger *zap.Logger) *Dispatcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{
		endpoints:  endpoints,
		timeout:    timeout,
		maxRetries: maxRetries,
		logger:     logger,
		queue:      make(chan Event, 1024),
		client:     &http.Client{Timeout: timeout},
		done:       make(chan struct{}),
		quit:       make(chan struct{}),
	}
}

// Start launches the delivery worker. Idempotent: calling it more than once
// spawns at most one worker.
func (d *Dispatcher) Start() {
	d.startOnce.Do(func() { go d.run() })
}

// Stop signals the worker to shut down and blocks until it has returned.
//
// Shutdown is best-effort: an in-flight delivery (including its bounded retry
// loop) is allowed to finish, but any events still buffered in the queue that
// have not yet been picked up are dropped — consistent with the drop-on-full
// contract. The queue itself is never closed; shutdown is signaled via the
// quit channel plus an atomic flag so a racing Emit can never send on a closed
// channel.
//
// Stop is idempotent and safe to call concurrently: subsequent calls are
// no-ops that simply wait for the (already-closed) done channel.
func (d *Dispatcher) Stop() {
	d.stopOnce.Do(func() {
		d.closed.Store(true)
		close(d.quit)
	})
	<-d.done
}

// Emit enqueues an event. Non-blocking: if the queue is full the event is
// dropped with a warning (back-pressure must never stall the indexer). After
// Stop has been called Emit is a harmless no-op.
func (d *Dispatcher) Emit(e Event) {
	if d.closed.Load() {
		return
	}
	select {
	case d.queue <- e:
	default:
		d.logger.Warn("webhook queue full; dropping event", zap.String("type", e.Type))
	}
}

func (d *Dispatcher) run() {
	defer close(d.done)
	for {
		select {
		case <-d.quit:
			return
		case e := <-d.queue:
			d.dispatch(e)
		}
	}
}

func (d *Dispatcher) dispatch(e Event) {
	body, err := json.Marshal(e)
	if err != nil {
		d.logger.Warn("webhook marshal failed", zap.Error(err))
		return
	}
	for _, ep := range d.endpoints {
		if !d.wants(ep, e.Type) {
			continue
		}
		d.deliver(ep, e.Type, body)
	}
}

func (d *Dispatcher) wants(ep Endpoint, eventType string) bool {
	if len(ep.Events) == 0 {
		return true
	}
	for _, ev := range ep.Events {
		if ev == eventType {
			return true
		}
	}
	return false
}

func (d *Dispatcher) deliver(ep Endpoint, eventType string, body []byte) {
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
		ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			cancel()
			d.logger.Warn("webhook request build failed", zap.Error(err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if ep.Secret != "" {
			req.Header.Set("X-Webhook-Signature", ComputeSignature(ep.Secret, body))
		}
		resp, err := d.client.Do(req)
		cancel()
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	d.logger.Warn("webhook delivery failed after retries",
		zap.String("url", ep.URL), zap.String("type", eventType))
}

// ComputeSignature returns the hex HMAC-SHA256 of body keyed by secret.
func ComputeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
