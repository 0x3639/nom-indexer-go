package health_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/health"
)

func TestHealthzAlwaysOK(t *testing.T) {
	srv := health.NewServer(func() health.Snapshot {
		return health.Snapshot{Ready: false, State: "stalled"}
	})
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/healthz code = %d, want 200", rr.Code)
	}
}

func TestHealthzBodyOk(t *testing.T) {
	srv := health.NewServer(func() health.Snapshot {
		return health.Snapshot{Ready: true, State: "synced"}
	})
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	body, _ := io.ReadAll(rr.Body)
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body not JSON: %v (%q)", err, body)
	}
	if got["status"] != "ok" {
		t.Fatalf(`got["status"] = %q, want "ok"`, got["status"])
	}
}

func TestReadyzReady(t *testing.T) {
	srv := health.NewServer(func() health.Snapshot {
		return health.Snapshot{Ready: true, State: "synced", NodeLabel: "primary", Drift: 0}
	})
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/readyz code = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), `"status":"ready"`) {
		t.Fatalf("/readyz ready body missing status:ready: %q", body)
	}
	if !strings.Contains(string(body), `"node":"primary"`) {
		t.Fatalf("/readyz ready body missing node label: %q", body)
	}
}

func TestReadyzNotReady(t *testing.T) {
	srv := health.NewServer(func() health.Snapshot {
		return health.Snapshot{Ready: false, State: "node_lagging", NodeLabel: "primary", Drift: 5}
	})
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz code = %d, want 503", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), `"status":"draining"`) {
		t.Fatalf("/readyz unready body missing status:draining: %q", body)
	}
	if !strings.Contains(string(body), `"state":"node_lagging"`) {
		t.Fatalf("/readyz unready body missing state: %q", body)
	}
}

func TestUnknownPath404(t *testing.T) {
	srv := health.NewServer(func() health.Snapshot {
		return health.Snapshot{Ready: true}
	})
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nonsense", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown path code = %d, want 404", rr.Code)
	}
}
