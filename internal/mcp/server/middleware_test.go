package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0x3639/nom-indexer-go/internal/auth"
)

func TestCORS_AllowsMCPPreflight(t *testing.T) {
	t.Parallel()

	h := CORS([]string{"https://app.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, Mcp-Session-Id")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want https://app.example.com", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_KeysByJWTSubject(t *testing.T) {
	t.Parallel()

	signer, err := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	tok, err := signer.Issue("alice", time.Hour, []string{"read"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	h := Auth(signer)(RateLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	for i, want := range []int{http.StatusNoContent, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d status = %d, want %d", i+1, rec.Code, want)
		}
		if want == http.StatusTooManyRequests {
			var body map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode rate limit body: %v", err)
			}
			errObj, _ := body["error"].(map[string]any)
			data, _ := errObj["data"].(map[string]any)
			if data["code"] != "rate_limited" {
				t.Fatalf("error.data.code = %v, want rate_limited", data["code"])
			}
		}
	}
}
