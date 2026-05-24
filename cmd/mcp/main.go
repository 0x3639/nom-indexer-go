// mcp is the nom-indexer-go MCP server. It exposes the same Postgres
// tables the REST API reads, but speaks the Model Context Protocol so
// AI agents (Claude Desktop, Claude Code, any tool-using LLM) can
// query them directly. See docs/mcp/ for the tool catalog and the
// remote-server configuration recipes.
//
// Process layout:
//
//   - Port MCP_PORT (default 8081) serves Streamable HTTP at POST /mcp,
//     JWT-protected via the same HS256 signer as the REST API.
//   - Port MCP_METRICS_PORT (default 9091) serves Prometheus /metrics
//     on a private listener so the public MCP endpoint doesn't leak
//     operational data.
//
// The tool catalog itself lives in internal/mcp/tools — this main
// does the lifecycle wiring only.
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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	mcpmetrics "github.com/0x3639/nom-indexer-go/internal/mcp/metrics"
	mcpserver "github.com/0x3639/nom-indexer-go/internal/mcp/server"
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
	secret := cfg.MCP.EffectiveJWTSecret(cfg.API.JWTSecret)
	if secret == "" {
		fmt.Fprintln(os.Stderr, "error: mcp.jwt_secret (env MCP_JWT_SECRET) or api.jwt_secret (env API_JWT_SECRET) is required")
		os.Exit(1)
	}

	logger, err := cfg.Logging.BuildLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	poolCtx, poolCancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := database.NewPool(poolCtx, &cfg.Database, logger)
	poolCancel()
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	repos := repository.NewRepositories(pool)

	signer, err := auth.NewSigner(secret)
	if err != nil {
		logger.Fatal("failed to initialize JWT signer", zap.Error(err))
	}

	m := mcpmetrics.New()

	srv := mcpserver.New(mcpserver.Deps{
		Repos:       repos,
		Logger:      logger,
		Middlewares: []mcp.Middleware{m.Middleware()},
	})
	handler := mcpserver.Auth(signer)(mcpserver.HTTPHandler(srv))

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)
	// Health probes live on the public MCP listener (not the metrics
	// listener) so external load balancers and k8s probes hit the same
	// port the JWT-protected MCP endpoint serves. Both are unauthenticated.
	mux.HandleFunc("/healthz", mcpserver.Healthz)
	mux.Handle("/readyz", mcpserver.Readyz(pool))

	mcpHTTP := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.MCP.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// No WriteTimeout — streaming responses (SSE) can hold the
		// connection open. The SDK manages per-call deadlines.
		IdleTimeout: 2 * time.Minute,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", m.Handler())
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.MCP.MetricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		logger.Info("MCP listening", zap.String("addr", mcpHTTP.Addr), zap.String("version", version))
		if err := mcpHTTP.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("mcp server: %w", err)
		}
	}()
	go func() {
		logger.Info("metrics listening", zap.String("addr", metricsSrv.Addr))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("metrics server: %w", err)
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
	if err := mcpHTTP.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown failed", zap.Error(err))
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("metrics graceful shutdown failed", zap.Error(err))
	}
	logger.Info("MCP stopped")
}
