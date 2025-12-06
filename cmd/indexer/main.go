package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/indexer"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	logger.Info("starting nom-indexer",
		zap.String("node_url", cfg.Node.WebSocketURL),
		zap.String("database", cfg.Database.Host))

	// Connect to database
	ctx := context.Background()
	pool, err := database.NewPool(ctx, &cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	logger.Info("connected to database")

	// Run migrations
	migrationsPath := "migrations"
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		migrationsPath = envPath
	}
	if err := database.RunMigrations(pool, migrationsPath, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	logger.Info("migrations complete")

	// Connect to Zenon node
	client, err := rpc_client.NewRpcClient(cfg.Node.WebSocketURL)
	if err != nil {
		logger.Fatal("failed to connect to node", zap.Error(err))
	}
	defer client.Stop()

	logger.Info("connected to Zenon node")

	// Create indexer
	idx := indexer.NewIndexer(client, pool, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
		cancel()
	}()

	// Run indexer
	if err := idx.Run(ctx); err != nil {
		if ctx.Err() != nil {
			logger.Info("indexer stopped gracefully")
		} else {
			logger.Fatal("indexer failed", zap.Error(err))
		}
	}
}
