package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/indexer"
)

func main() {
	// Load configuration first so the logger reflects the user's choices.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger, err := cfg.Logging.BuildLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

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
	if migrationErr := database.RunMigrations(pool, migrationsPath, logger); migrationErr != nil {
		logger.Fatal("failed to run migrations", zap.Error(migrationErr))
	}

	logger.Info("migrations complete")

	// Connect to Zenon node
	client, err := rpc_client.NewRpcClient(cfg.Node.WebSocketURL)
	if err != nil {
		logger.Fatal("failed to connect to node", zap.Error(err))
	}
	defer client.Stop()

	logger.Info("connected to Zenon node")

	votingInterval, err := indexer.ParseCronInterval(cfg.Cron.VotingActivityInterval, 10*time.Minute)
	if err != nil {
		logger.Fatal("invalid cron.voting_activity_interval", zap.Error(err))
	}
	tokenHoldersInterval, err := indexer.ParseCronInterval(cfg.Cron.TokenHoldersInterval, 10*time.Minute)
	if err != nil {
		logger.Fatal("invalid cron.token_holders_interval", zap.Error(err))
	}

	idx := indexer.NewIndexerWithCron(client, pool, logger, indexer.CronConfig{
		VotingActivityInterval: votingInterval,
		TokenHoldersInterval:   tokenHoldersInterval,
	})

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

	// Run backfill if enabled (fills gaps from previous runs before syncing)
	if cfg.BackfillOnStartup {
		logger.Info("backfill on startup enabled, checking for gaps")
		if err := idx.Backfill(ctx); err != nil {
			logger.Error("backfill failed", zap.Error(err))
			// Don't fatal - continue with normal sync
		}
	}

	// Run indexer
	if err := idx.Run(ctx); err != nil {
		if ctx.Err() != nil {
			logger.Info("indexer stopped gracefully")
		} else {
			logger.Fatal("indexer failed", zap.Error(err))
		}
	}
}
