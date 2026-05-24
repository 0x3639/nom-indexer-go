package middleware

import (
	"context"
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

// subHolder is the per-request mutable cell the Auth middleware writes
// the JWT subject into. The Logger middleware allocates one and seeds
// the request context with a pointer to it BEFORE calling
// next.ServeHTTP — so when Auth writes through that pointer, the
// Logger sees the value after the handler returns.
//
// We can't read claims directly from r.Context() after next.ServeHTTP
// because Auth attaches them via r.WithContext(ctx); contexts only
// flow downstream, not back up to the caller's *Request.
type subHolder struct {
	sub string
}

type subHolderKey struct{}

// SetSubject records the JWT subject on the per-request holder seeded
// by the Logger middleware. Called by the Auth middleware; safe to call
// when no holder is present (no-op in that case, e.g. tests that mount
// Auth without Logger).
func SetSubject(ctx context.Context, sub string) {
	if h, ok := ctx.Value(subHolderKey{}).(*subHolder); ok {
		h.sub = sub
	}
}

// Logger emits one structured log line per request after the handler
// returns. Fields: method, path, status, duration_ms, bytes, request_id,
// sub (JWT subject — populated by Auth middleware via SetSubject when
// the request was authenticated).
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			holder := &subHolder{}
			ctx := context.WithValue(r.Context(), subHolderKey{}, holder)
			next.ServeHTTP(rec, r.WithContext(ctx))

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Int64("duration_ms", time.Since(start).Milliseconds()),
				zap.Int("bytes", rec.bytes),
				zap.String("request_id", RequestIDFromContext(r.Context())),
			}
			if holder.sub != "" {
				fields = append(fields, zap.String("sub", holder.sub))
			}
			logger.Info("http", fields...)
		})
	}
}
