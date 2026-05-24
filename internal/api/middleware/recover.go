package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
)

// Recover catches panics in downstream handlers, logs the stack trace,
// and returns 500 application/problem+json. Without this, a panic kills
// the goroutine and the connection drops mid-response.
func Recover(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// httpx.WriteProblem writes a fixed problem+json response and
			// does not perform any context-bound I/O; ignore contextcheck.
			defer func() { //nolint:contextcheck // fixed response, no context required
				if rv := recover(); rv != nil {
					logger.Error("handler panic",
						zap.Any("recover", rv),
						zap.String("request_id", RequestIDFromContext(r.Context())),
						zap.String("path", r.URL.Path),
						zap.ByteString("stack", debug.Stack()))
					httpx.WriteProblem(w, http.StatusInternalServerError,
						"internal_error", "the server encountered an unexpected condition")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
