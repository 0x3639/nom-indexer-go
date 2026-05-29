// Package health implements the indexer's own HTTP health server.
// /healthz reports process liveness only; /readyz reflects the
// watchdog's last classification so external monitoring and docker
// compose healthchecks can react to drift without restarting the
// indexer.
package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Snapshot is the minimal view of indexer health surfaced by /readyz.
// Built by the watchdog goroutine; consumed via a callback the indexer
// passes to NewServer.
type Snapshot struct {
	Ready     bool   `json:"-"`
	State     string `json:"state,omitempty"`
	NodeLabel string `json:"node,omitempty"`
	Drift     int64  `json:"drift,omitempty"`
}

// Server holds a configured http.Handler. Build one with NewServer
// and call ListenAndServe to start a listener on the given address.
type Server struct {
	Handler http.Handler
}

// NewServer wires /healthz (always 200) and /readyz (200 when
// snapshot().Ready, else 503). The snapshot callback is invoked on
// every /readyz request; the indexer's HealthSnapshot() implementation
// already takes a brief lock, so this is safe for concurrent traffic.
func NewServer(snapshot func() Snapshot) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		snap := snapshot()
		body := map[string]any{
			"status": "ready",
			"state":  snap.State,
			"node":   snap.NodeLabel,
			"drift":  snap.Drift,
		}
		code := http.StatusOK
		if !snap.Ready {
			code = http.StatusServiceUnavailable
			body["status"] = "draining"
		}
		writeJSON(w, code, body)
	})
	return &Server{Handler: mux}
}

// ListenAndServe binds the handler to addr (host:port). Blocks until
// the server stops; returns http.ErrServerClosed on clean shutdown.
func (s *Server) ListenAndServe(addr string) error {
	return (&http.Server{
		Addr:              addr,
		Handler:           s.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}).ListenAndServe()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_, _ = fmt.Fprintf(w, `{"error":%q}`, err.Error())
	}
}
