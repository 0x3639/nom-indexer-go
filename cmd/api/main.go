// api is the nom-indexer-go HTTP API server.
//
// It exposes read-only endpoints over the database the indexer fills. See
// docs/api/ for the full specification, including authentication (HS256
// JWT) and the per-endpoint contract in docs/api/openapi.yaml.
//
// This main composes the middleware stack and a chi router; per-domain
// handlers land in internal/api/handlers in subsequent milestones.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	apimw "github.com/0x3639/nom-indexer-go/internal/api/middleware"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	if cfg.API.JWTSecret == "" {
		fmt.Fprintln(os.Stderr, "error: api.jwt_secret (env API_JWT_SECRET) is required")
		os.Exit(1)
	}

	logger, err := cfg.Logging.BuildLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	signer, err := auth.NewSigner(cfg.API.JWTSecret)
	if err != nil {
		logger.Fatal("failed to initialize JWT signer", zap.Error(err))
	}

	r := newRouter(cfg, logger, signer)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.API.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("API listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Fatal("server error", zap.Error(err))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown failed", zap.Error(err))
	}
	logger.Info("API stopped")
}

// newRouter builds the chi router with the middleware stack. Subsequent
// milestones add domain handlers under /api/v1/.
func newRouter(cfg *config.Config, logger *zap.Logger, signer *auth.Signer) http.Handler {
	r := chi.NewRouter()

	// Always-on middleware (every route, including /healthz).
	r.Use(apimw.RequestID)
	r.Use(apimw.Logger(logger))
	r.Use(apimw.Recover(logger))
	r.Use(apimw.CORS(cfg.API.CORSAllowedOriginsList()))

	// Unauthenticated routes.
	r.Get("/healthz", healthz)
	// /readyz, /metrics, /api/v1/{...} land in later milestones.

	// Authenticated /api/v1 subtree.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.Auth(signer))
		r.Use(apimw.RateLimit(cfg.API.RateLimitPerMinute))

		// Placeholder; real handlers land in M4+.
		r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"version": "dev"})
		})
	})

	return r
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
