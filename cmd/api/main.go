// api is the nom-indexer-go HTTP API server.
//
// It exposes read-only endpoints over the database the indexer fills. See
// docs/api/ for the full specification, including authentication (HS256
// JWT) and the per-endpoint contract in docs/api/openapi.yaml.
//
// This main composes the database pool, repositories, middleware stack,
// and router — the route table itself lives in internal/api/router so
// the router test can drift-check it against the OpenAPI spec.
//
// A second http.Server (port 9090 by default) serves Prometheus metrics
// on /metrics; keeping it on a separate listener means the public API
// surface never accidentally exposes operational data.
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

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/metrics"
	"github.com/0x3639/nom-indexer-go/internal/api/router"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
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
	m := metrics.New()

	// Stream hubs: dedicated pgx.Conn (NOT from the pool) each holds a
	// LISTEN on its respective channel for the process lifetime.
	// Constructed here so the router can route the WS endpoints; the
	// goroutines that actually run them are started after the router
	// is built so a hub failure can't race the HTTP listener.
	connectStreamConn := func(ctx context.Context) (*pgx.Conn, error) {
		return pgx.Connect(ctx, cfg.Database.ConnectionString())
	}
	hub := stream.New(stream.Config[*dto.Momentum]{
		ConnectFn:   connectStreamConn,
		Logger:      logger,
		ChannelName: "momentum_new",
		Unmarshal:   stream.UnmarshalJSON[dto.Momentum](),
	})
	txHub := stream.New(stream.Config[*dto.AccountBlock]{
		ConnectFn:   connectStreamConn,
		Logger:      logger,
		ChannelName: "account_block_new",
		Unmarshal:   stream.UnmarshalJSON[dto.AccountBlock](),
	})

	r := router.New(router.Deps{
		Repos:              repos,
		Signer:             signer,
		Logger:             logger,
		Pool:               pool,
		Hub:                hub,
		TxHub:              txHub,
		Metrics:            m.Middleware,
		CORSAllowedOrigins: cfg.API.CORSAllowedOriginsList(),
		RateLimitPerMinute: cfg.API.RateLimitPerMinute,
		Version:            version,
	})

	apiSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.API.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", m.Handler())
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.API.MetricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		logger.Info("API listening", zap.String("addr", apiSrv.Addr))
		if err := apiSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()
	go func() {
		logger.Info("metrics listening", zap.String("addr", metricsSrv.Addr))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()
	// Stream hubs run alongside the HTTP listeners. A hub failure
	// (connection drop, LISTEN error) downgrades only the matching
	// streaming endpoint to 5xx for new subscribers; it does NOT bring
	// down the REST API or the other hub — log and continue.
	go func() {
		if err := hub.Run(ctx); err != nil {
			logger.Error("momentum stream hub exited; /api/v1/momentums/stream is degraded",
				zap.Error(err))
		}
	}()
	go func() {
		if err := txHub.Run(ctx); err != nil {
			logger.Error("transaction stream hub exited; /api/v1/transactions/stream is degraded",
				zap.Error(err))
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
	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("api graceful shutdown failed", zap.Error(err))
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("metrics graceful shutdown failed", zap.Error(err))
	}
	logger.Info("API stopped")
}
