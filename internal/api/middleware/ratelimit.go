package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
)

// RateLimit returns a middleware that limits each JWT subject to
// `perMinute` requests in any sliding 60-second window. Requests over
// the limit get RFC 7807 application/problem+json with status 429.
//
// The middleware chain places Auth before RateLimit, so unauthenticated
// requests never reach this code — they are short-circuited with 401
// in Auth. The IP-fallback path in keyBySubjectThenIP is therefore
// dead under the current ordering, kept defensively in case someone
// reorders middleware in the future or mounts RateLimit on an
// unauthenticated subtree.
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	if perMinute <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return httprate.Limit(
		perMinute,
		time.Minute,
		httprate.WithKeyFuncs(keyBySubjectThenIP),
		httprate.WithLimitHandler(http.HandlerFunc(rateLimited)),
	)
}

func keyBySubjectThenIP(r *http.Request) (string, error) {
	if claims := ClaimsFromContext(r.Context()); claims != nil && claims.Subject != "" {
		return "sub:" + claims.Subject, nil
	}
	return httprate.KeyByIP(r)
}

func rateLimited(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteProblem(w, http.StatusTooManyRequests, "rate_limited",
		"too many requests; slow down")
}
