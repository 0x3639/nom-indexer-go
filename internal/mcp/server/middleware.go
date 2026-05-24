package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

const (
	headerRequestID       = "X-Request-Id"
	headerSessionID       = "Mcp-Session-Id"
	headerProtocolVersion = "MCP-Protocol-Version"
	headerLastEventID     = "Last-Event-ID"
)

// CORS returns the MCP transport CORS middleware. An empty allowlist is a
// no-op: non-browser clients continue to work, while browsers block
// cross-origin calls because no Access-Control-Allow-Origin header is emitted.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			headerRequestID,
			headerSessionID,
			headerProtocolVersion,
			headerLastEventID,
		},
		ExposedHeaders: []string{
			headerRequestID,
			headerSessionID,
			headerProtocolVersion,
		},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

// RateLimit caps authenticated MCP requests per JWT subject. Auth must wrap
// this middleware so ClaimsFromContext can see the verified subject.
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"error": map[string]any{
			"code":    -32002,
			"message": "too many requests; slow down",
			"data":    map[string]any{"code": "rate_limited"},
		},
		"id": nil,
	})
}
