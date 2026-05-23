package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/auth"
)

// noopHandler writes 200 OK with an empty body.
var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRequestID_SetsHeaderAndContext(t *testing.T) {
	var captured string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(rr, req)

	if captured == "" {
		t.Fatal("expected RequestIDFromContext to return non-empty")
	}
	if got := rr.Header().Get(HeaderRequestID); got != captured {
		t.Errorf("response header = %q, context = %q; want equal", got, captured)
	}
}

func TestRequestID_EchoesClientHeader(t *testing.T) {
	const supplied = "client-supplied-id"
	h := RequestID(noopHandler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderRequestID, supplied)
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get(HeaderRequestID); got != supplied {
		t.Errorf("response header = %q, want %q", got, supplied)
	}
}

func TestRecover_TurnsPanicInto500Problem(t *testing.T) {
	logger := zap.NewNop()
	h := Recover(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("kaboom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", ct)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if p.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", p.Code)
	}
}

func TestLogger_EmitsOneLineWithExpectedFields(t *testing.T) {
	core, recorded := newObservedCore()
	logger := zap.New(core)

	h := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", "/widgets/42", nil))

	if recorded.Len() != 1 {
		t.Fatalf("expected 1 log line, got %d", recorded.Len())
	}
	entry := recorded.All()[0]
	if entry.Message != "http" {
		t.Errorf("message = %q, want http", entry.Message)
	}
	want := map[string]any{
		"method": "PUT",
		"path":   "/widgets/42",
		"status": int64(201),
	}
	for k, v := range want {
		got := entry.ContextMap()[k]
		if got != v {
			t.Errorf("field %s = %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}
}

// TestLogger_CapturesJWTSubject covers the holder-pointer wiring
// between Auth and Logger. Auth attaches claims via r.WithContext to a
// downstream request, so Logger cannot read them back from its own *r;
// SetSubject writes through the holder pointer that Logger seeded in
// the outer context. Without that wiring the "sub" field never appears.
func TestLogger_CapturesJWTSubject(t *testing.T) {
	core, recorded := newObservedCore()
	logger := zap.New(core)
	signer, _ := auth.NewSigner("secret")
	tok, _ := signer.Issue("alice", time.Hour, []string{"read"})

	// Logger → Auth → handler — the production middleware order.
	h := Logger(logger)(Auth(signer)(noopHandler))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if recorded.Len() != 1 {
		t.Fatalf("expected 1 log line, got %d", recorded.Len())
	}
	got := recorded.All()[0].ContextMap()["sub"]
	if got != "alice" {
		t.Errorf("log field sub = %v, want alice", got)
	}
}

func TestAuth_RejectsMissingHeader(t *testing.T) {
	signer, _ := auth.NewSigner("secret")
	h := Auth(signer)(noopHandler)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAuth_RejectsNonBearer(t *testing.T) {
	signer, _ := auth.NewSigner("secret")
	h := Auth(signer)(noopHandler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAuth_RejectsBadToken(t *testing.T) {
	signer, _ := auth.NewSigner("secret")
	h := Auth(signer)(noopHandler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAuth_PassesValidTokenAndAttachesClaims(t *testing.T) {
	signer, _ := auth.NewSigner("secret")
	tok, _ := signer.Issue("alice", time.Hour, []string{"read"})

	var seenSub string
	h := Auth(signer)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		c := ClaimsFromContext(r.Context())
		if c != nil {
			seenSub = c.Subject
		}
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if seenSub != "alice" {
		t.Errorf("sub in context = %q, want alice", seenSub)
	}
}

func TestBearerToken_Tolerant(t *testing.T) {
	tests := []struct {
		header    string
		want      string
		wantFound bool
		wantOK    bool
	}{
		{"", "", false, false},
		{"Bearer ", "", true, false},
		{"Basic xxx", "", true, false},
		{"Bearer abc", "abc", true, true},
		{"bearer abc", "abc", true, true}, // case-insensitive scheme
		{"Bearer  abc  ", "abc", true, true},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/", nil)
		if tt.header != "" {
			r.Header.Set("Authorization", tt.header)
		}
		got, found, ok := bearerToken(r)
		if found != tt.wantFound || ok != tt.wantOK || strings.TrimSpace(got) != tt.want {
			t.Errorf("bearerToken(%q) = (%q, found=%v, ok=%v), want (%q, found=%v, ok=%v)",
				tt.header, got, found, ok, tt.want, tt.wantFound, tt.wantOK)
		}
	}
}

func TestAuth_DistinctErrorCodes(t *testing.T) {
	signer, _ := auth.NewSigner("secret")

	// Expired token — Issue refuses ttl<=0, so synthesize directly.
	expiredTok, _ := signer.Issue("alice", time.Hour, []string{"read"})
	// Parse it to confirm it's valid, then create one whose exp is in the past.
	expired := makeExpiredToken(t, "secret")

	cases := []struct {
		name     string
		header   string
		wantCode string
	}{
		{"missing", "", "missing_token"},
		{"malformed_scheme", "Basic xxx", "invalid_token"},
		{"malformed_no_space", "Bearer", "invalid_token"},
		{"invalid_signature", "Bearer not-a-token", "invalid_token"},
		{"valid", "Bearer " + expiredTok, ""}, // passes — no problem body
		{"expired", "Bearer " + expired, "expired_token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := Auth(signer)(noopHandler)
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			h.ServeHTTP(rr, req)
			if tc.wantCode == "" {
				if rr.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
				}
				return
			}
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body=%s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), `"code":"`+tc.wantCode+`"`) {
				t.Errorf("body missing code=%q: %s", tc.wantCode, rr.Body.String())
			}
		})
	}
}

func makeExpiredToken(t *testing.T, secret string) string {
	t.Helper()
	// Mint a token via the signer with the smallest legal TTL, then sleep
	// past its expiry. golang-jwt/jwt's default leeway is 0, so a 1s TTL
	// + a brief sleep is enough.
	s, err := auth.NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	tok, err := s.Issue("alice", time.Nanosecond, []string{"read"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Give the clock a tick so exp is unambiguously in the past.
	time.Sleep(2 * time.Millisecond)
	return tok
}

func TestCORS_NoopWhenAllowlistEmpty(t *testing.T) {
	h := CORS(nil)(noopHandler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header when allowlist is empty, got %q", got)
	}
}

func TestCORS_AllowsListedOrigin(t *testing.T) {
	h := CORS([]string{"https://example.com"})(noopHandler)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://example.com", got)
	}
}

func TestRateLimit_AllowsBelowThreshold(t *testing.T) {
	h := RateLimit(10)(noopHandler)
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rr.Code)
		}
	}
}

func TestRateLimit_DisabledWhenZero(t *testing.T) {
	h := RateLimit(0)(noopHandler)
	for i := 0; i < 100; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("zero limit must disable: status = %d", rr.Code)
		}
	}
}
