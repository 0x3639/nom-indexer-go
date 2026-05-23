package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// statusRecorder wraps http.ResponseWriter to capture the status code and
// the number of bytes written for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Logger emits one structured log line per request after the handler
// returns. Fields: method, path, status, duration_ms, bytes, request_id,
// sub (JWT subject when AttachClaims has populated it).
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Int64("duration_ms", time.Since(start).Milliseconds()),
				zap.Int("bytes", rec.bytes),
				zap.String("request_id", RequestIDFromContext(r.Context())),
			}
			if claims := ClaimsFromContext(r.Context()); claims != nil {
				fields = append(fields, zap.String("sub", claims.Subject))
			}
			logger.Info("http", fields...)
		})
	}
}
