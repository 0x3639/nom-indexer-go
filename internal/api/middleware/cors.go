package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns a middleware configured from the comma-separated allowlist.
// An empty allowlist disables cross-origin requests entirely (the default).
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		// No-op middleware: same-origin requests still work; cross-origin
		// fail at the browser because no Access-Control-Allow-Origin is set.
		return func(next http.Handler) http.Handler { return next }
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", HeaderRequestID},
		ExposedHeaders:   []string{HeaderRequestID},
		AllowCredentials: false,
		MaxAge:           300,
	})
}
