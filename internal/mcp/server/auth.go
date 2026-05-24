package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/0x3639/nom-indexer-go/internal/auth"
)

// claimsKey is the context key for verified JWT claims.
type claimsKey struct{}

// ClaimsFromContext returns the *auth.Claims attached by Auth, or nil.
// Tools can call this to access the calling subject for per-request
// logic (audit logs, per-subject filtering, etc.).
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey{}).(*auth.Claims)
	return c
}

// Auth returns middleware that verifies an HS256 JWT bearer on every
// request. Accepts the token from either:
//
//   - Authorization: Bearer <token>     (CLI / server-side clients)
//   - ?token=<token> query parameter    (browsers — same fallback the
//     WS streams use because the WebSocket API can't send custom
//     headers, and the same pragma applies to some MCP clients that
//     embed an iframe)
//
// On failure responds with HTTP 401 and a JSON-RPC error envelope so
// MCP clients see a structured payload, not just a status code.
func Auth(signer *auth.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, found, ok := bearerToken(r)
			if !found {
				writeAuthError(w, "missing_token",
					"missing JWT (Authorization header or ?token= query)")
				return
			}
			if !ok {
				writeAuthError(w, "invalid_token",
					"malformed Authorization header (want: Bearer <token>)")
				return
			}
			claims, err := signer.Verify(tok)
			if err != nil {
				code := "invalid_token"
				if errors.Is(err, jwt.ErrTokenExpired) {
					code = "expired_token"
				}
				writeAuthError(w, code, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token from Authorization or ?token=.
// Returns (token, found, ok) — same shape as internal/api/middleware
// to keep the two auth layers consistent.
func bearerToken(r *http.Request) (token string, found bool, ok bool) {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "Bearer "
		if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			return "", true, false
		}
		return strings.TrimSpace(h[len(prefix):]), true, true
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t, true, true
	}
	return "", false, false
}

// writeAuthError emits a JSON-RPC 2.0 error frame at HTTP 401. The
// MCP spec lets transports return non-200 status codes for transport
// failures (auth is transport-level, not protocol-level), but
// well-behaved clients prefer a structured body to parse.
func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"error": map[string]any{
			"code":    -32001, // JSON-RPC implementation-defined server error range
			"message": message,
			"data":    map[string]any{"code": code},
		},
		"id": nil,
	})
}
