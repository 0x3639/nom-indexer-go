// api is the nom-indexer-go HTTP API server.
//
// It exposes read-only endpoints over the database the indexer fills. See
// docs/api/ for the full specification, including authentication (HS256
// JWT) and the per-endpoint contract in docs/api/openapi.yaml.
//
// This main composes the database pool, repositories, middleware stack,
// and router — the route table itself lives in internal/api/router so
// the router test can drift-check it against the OpenAPI spec.
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

	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/router"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// version is overridable at build time via -ldflags "-X main.version=…".
var version = "dev"

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

	poolCtx, poolCancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := database.NewPool(poolCtx, &cfg.Database, logger)
	poolCancel()
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	repos := repository.NewRepositories(pool)

	r := router.New(router.Deps{
		Repos:              repos,
		Signer:             signer,
		Logger:             logger,
		CORSAllowedOrigins: cfg.API.CORSAllowedOriginsList(),
		RateLimitPerMinute: cfg.API.RateLimitPerMinute,
		Version:            version,
	})

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
