package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/auth"
)

// claimsKey is the context key for the verified JWT claims.
type claimsKey struct{}

// Distinct problem codes documented in docs/api/auth.md. Splitting them
// here keeps the docs honest: a missing header is not the same failure
// as a parseable-but-expired token, and clients can use the code to
// decide whether to prompt for credentials, refresh a stored token, or
// surface "your link is bad".
const (
	codeMissingToken = "missing_token"
	codeInvalidToken = "invalid_token"
	codeExpiredToken = "expired_token"
)

// Auth returns middleware that verifies the Authorization: Bearer <token>
// header against the given signer and attaches the parsed *auth.Claims to
// the request context. Requests without a token, with a malformed header,
// or with an invalid token receive 401 application/problem+json with a
// `code` field that distinguishes the three cases.
func Auth(signer *auth.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, found, ok := bearerToken(r)
			if !found {
				httpx.WriteProblem(w, http.StatusUnauthorized, codeMissingToken,
					"missing Authorization header (want: Bearer <token>)")
				return
			}
			if !ok {
				httpx.WriteProblem(w, http.StatusUnauthorized, codeInvalidToken,
					"malformed Authorization header (want: Bearer <token>)")
				return
			}
			claims, err := signer.Verify(tok)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					httpx.WriteProblem(w, http.StatusUnauthorized, codeExpiredToken,
						"token expired")
					return
				}
				httpx.WriteProblem(w, http.StatusUnauthorized, codeInvalidToken,
					"invalid token")
				return
			}
			// Surface the subject to the Logger middleware via the
			// pointer-holder it seeded into the outer context. Reading
			// claims from r.Context() after next.ServeHTTP would not
			// work: Auth attaches claims to a downstream ctx via
			// r.WithContext, which never flows back to the logger.
			SetSubject(r.Context(), claims.Subject)
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
// header.
//
// Return values:
//   - found=false: no Authorization header present
//   - found=true, ok=false: header present but malformed
//   - found=true, ok=true: token extracted
func bearerToken(r *http.Request) (token string, found bool, ok bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false, false
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", true, false
	}
	return strings.TrimSpace(h[len(prefix):]), true, true
}
