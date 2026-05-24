package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz_OK(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body: want status=ok, got %v", body)
	}
}

func TestReadyz_NilPool_IsReady(t *testing.T) {
	t.Parallel()
	// nil pool is treated as ready (test mode). Real DB-backed
	// behavior is covered by the integration tests, which boot a
	// migrated test DB. Keeping the nil-pool path tested here ensures
	// the handler does not panic when wired up without a pool.
	rec := httptest.NewRecorder()
	Readyz(nil)(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("body: want status=ready, got %v", body)
	}
}
