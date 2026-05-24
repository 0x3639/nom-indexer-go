// Package metrics owns the Prometheus registry and the HTTP middleware
// that observes request count and latency for the API.
//
// Metrics are exported on a separate listener (port 9090 by default) so
// the public API surface doesn't leak operational data. The middleware
// labels each observation with method, status, and the chi route
// pattern — never the raw path — so cardinality stays bounded.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns the prometheus.Registry and its registered collectors.
// Build one via New(); pass Metrics.Middleware() to the chi router and
// hand Metrics.Handler() to the metrics http.Server.
//
// Note: chi short-circuits unmatched routes before middleware runs, so
// the request counter only covers requests that hit a registered
// handler — 404s are not observed. That keeps label cardinality bounded.
type Metrics struct {
	registry      *prometheus.Registry
	requestsTotal *prometheus.CounterVec
	requestDur    *prometheus.HistogramVec
}

// New constructs a Metrics with the standard process + Go runtime
// collectors plus the API's request counter and latency histogram.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nom_api",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests handled by the API, labeled by method, route, and status.",
	}, []string{"method", "route", "status"})

	requestDur := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nom_api",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests handled by the API, in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	reg.MustRegister(requestsTotal, requestDur)

	return &Metrics{
		registry:      reg,
		requestsTotal: requestsTotal,
		requestDur:    requestDur,
	}
}

// Handler returns the promhttp.HandlerFor for the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Middleware observes every request and labels it with the chi route
// pattern (e.g. "/api/v1/momentums/{height}") rather than the raw URL,
// so each templated route maps to one label set.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			// Defensive: routes registered without chi (e.g. an mux.Handle
			// wrapper) won't populate RoutePattern. Bucket those rather
			// than expanding cardinality with raw URLs.
			route = "other"
		}
		status := strconv.Itoa(sr.status)
		m.requestsTotal.WithLabelValues(r.Method, route, status).Inc()
		m.requestDur.WithLabelValues(r.Method, route, status).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
		s.ResponseWriter.WriteHeader(code)
	}
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}
