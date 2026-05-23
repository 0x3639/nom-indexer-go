package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/auth"
)

// claimsKey is the context key for the verified JWT claims.
type claimsKey struct{}

// Auth returns middleware that verifies the Authorization: Bearer <token>
// header against the given signer and attaches the parsed *auth.Claims to
// the request context. Requests without a token, with a malformed header,
// or with an invalid token receive 401 application/problem+json.
func Auth(signer *auth.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearerToken(r)
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, "unauthorized",
					"missing or malformed Authorization header (want: Bearer <token>)")
				return
			}
			claims, err := signer.Verify(tok)
			if err != nil {
				httpx.WriteProblem(w, http.StatusUnauthorized, "unauthorized",
					"invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns the *auth.Claims attached by Auth, or nil.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey{}).(*auth.Claims)
	return c
}

// bearerToken extracts the token from an Authorization: Bearer <token>
// header. Returns ("", false) if the header is missing or malformed.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
