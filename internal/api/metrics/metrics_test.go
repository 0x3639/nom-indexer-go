package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMetrics_RouteLabel(t *testing.T) {
	m := New()

	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/api/v1/momentums/{height}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Two requests against the same templated route should collapse to one
	// label set with count=2 — proving the middleware uses the chi route
	// pattern, not the raw path.
	for _, path := range []string{"/api/v1/momentums/1", "/api/v1/momentums/2"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status=%d", path, w.Code)
		}
	}

	body := scrape(t, m)
	if !strings.Contains(body, `nom_api_http_requests_total{method="GET",route="/api/v1/momentums/{height}",status="200"} 2`) {
		t.Errorf("expected counter with templated route label and value 2; got:\n%s", body)
	}
}

func TestMetrics_DurationHistogram(t *testing.T) {
	m := New()
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	body := scrape(t, m)
	if !strings.Contains(body, `nom_api_http_request_duration_seconds_count{method="GET",route="/x",status="418"} 1`) {
		t.Errorf("expected duration histogram observation; got:\n%s", body)
	}
}

func TestMetrics_StatusFromExplicitWriteHeader(t *testing.T) {
	m := New()
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/y", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/y", nil))

	body := scrape(t, m)
	if !strings.Contains(body, `status="401"`) {
		t.Errorf("expected status=\"401\" label; got:\n%s", body)
	}
}

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return w.Body.String()
}
