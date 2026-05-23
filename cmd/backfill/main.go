// backfill is a one-shot tool that fills gaps in the momentums table by
// fetching missing/incomplete heights from the node and reprocessing them.
// It delegates the actual work to indexer.Backfill so the gap-finding query
// and processing path stay in a single place.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/indexer"
)

func main() {
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

	logger.Info("starting backfill tool",
		zap.String("node_url", cfg.Node.WebSocketURL),
		zap.String("database", cfg.Database.Host))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
		cancel()
	}()

	pool, err := database.NewPool(ctx, &cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	logger.Info("connected to database")

	client, err := rpc_client.NewRpcClient(cfg.Node.WebSocketURL)
	if err != nil {
		logger.Fatal("failed to connect to node", zap.Error(err))
	}
	defer client.Stop()

	logger.Info("connected to Zenon node")

	idx := indexer.NewIndexer(client, pool, logger)

	// Pillar/sentinel/project cache must be warm so processed momentums can
	// resolve owner addresses and embedded contract events.
	logger.Info("loading cached data from node")
	if err := idx.UpdateCachedDataPublic(ctx); err != nil {
		logger.Warn("failed to load cached data, some processing may be incomplete", zap.Error(err))
	}

	if err := idx.Backfill(ctx); err != nil {
		if ctx.Err() != nil {
			logger.Info("backfill stopped gracefully")
			return
		}
		logger.Fatal("backfill failed", zap.Error(err))
	}

	logger.Info("backfill complete")
}
