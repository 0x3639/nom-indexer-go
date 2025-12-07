package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/indexer"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	logger.Info("starting backfill tool",
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

	// Connect to Zenon node
	client, err := rpc_client.NewRpcClient(cfg.Node.WebSocketURL)
	if err != nil {
		logger.Fatal("failed to connect to node", zap.Error(err))
	}
	defer client.Stop()

	logger.Info("connected to Zenon node")

	// Create indexer (we'll use its processMomentum method)
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

	// Load cached data (pillars, etc.) needed for processing
	logger.Info("loading cached data from node")
	if err := idx.UpdateCachedDataPublic(ctx); err != nil {
		logger.Warn("failed to load cached data, some processing may be incomplete", zap.Error(err))
	}

	// Find and backfill missing momentums
	if err := backfillMissingMomentums(ctx, pool, client, idx, logger); err != nil {
		if ctx.Err() != nil {
			logger.Info("backfill stopped gracefully")
		} else {
			logger.Fatal("backfill failed", zap.Error(err))
		}
	}

	logger.Info("backfill complete")
}

func backfillMissingMomentums(ctx context.Context, pool *pgxpool.Pool, client *rpc_client.RpcClient, idx *indexer.Indexer, logger *zap.Logger) error {
	// Find missing momentum heights OR momentums with missing account blocks
	query := `
		WITH expected AS (
			SELECT generate_series(1::bigint, (SELECT MAX(height) FROM momentums)) as height
		),
		missing_momentums AS (
			SELECT e.height
			FROM expected e
			LEFT JOIN momentums m ON e.height = m.height
			WHERE m.height IS NULL
		),
		incomplete_momentums AS (
			SELECT m.height
			FROM momentums m
			LEFT JOIN (
				SELECT momentum_height, COUNT(*) as actual_txs
				FROM account_blocks
				GROUP BY momentum_height
			) ab ON m.height = ab.momentum_height
			WHERE m.tx_count > 0 AND COALESCE(ab.actual_txs, 0) < m.tx_count
		)
		SELECT height FROM missing_momentums
		UNION
		SELECT height FROM incomplete_momentums
		ORDER BY height
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query missing momentums: %w", err)
	}
	defer rows.Close()

	var missingHeights []uint64
	for rows.Next() {
		var height uint64
		if err := rows.Scan(&height); err != nil {
			return fmt.Errorf("failed to scan height: %w", err)
		}
		missingHeights = append(missingHeights, height)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	if len(missingHeights) == 0 {
		logger.Info("no missing or incomplete momentums found")
		return nil
	}

	logger.Info("found missing or incomplete momentums", zap.Int("count", len(missingHeights)))

	// Process each missing momentum
	for i, height := range missingHeights {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		logger.Info("backfilling momentum",
			zap.Uint64("height", height),
			zap.Int("progress", i+1),
			zap.Int("total", len(missingHeights)))

		// Fetch momentum from node
		momentums, err := client.LedgerApi.GetMomentumsByHeight(height, 1)
		if err != nil {
			logger.Error("failed to fetch momentum",
				zap.Uint64("height", height),
				zap.Error(err))
			continue
		}

		if momentums == nil || len(momentums.List) == 0 {
			logger.Warn("momentum not found on node",
				zap.Uint64("height", height))
			continue
		}

		// Process the momentum using the indexer
		if processErr := idx.ProcessMomentumPublic(ctx, momentums.List[0]); processErr != nil {
			logger.Error("failed to process momentum",
				zap.Uint64("height", height),
				zap.Error(processErr))
			// Don't continue - still try to insert the momentum directly
		}

		// Verify the momentum was inserted, if not insert it directly
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM momentums WHERE height = $1)", height).Scan(&exists)
		if err != nil {
			logger.Error("failed to check if momentum exists", zap.Uint64("height", height), zap.Error(err))
			continue
		}

		if !exists {
			// Insert momentum directly
			m := momentums.List[0]
			_, err = pool.Exec(ctx, `
				INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
				VALUES ($1, $2, $3, $4, $5, '', '')
				ON CONFLICT (height) DO NOTHING`,
				m.Height, m.Hash.String(), int64(m.TimestampUnix), len(m.Content), m.Producer.String())
			if err != nil {
				logger.Error("failed to insert momentum directly",
					zap.Uint64("height", height),
					zap.Error(err))
				continue
			}
			logger.Info("inserted momentum directly (batch had errors)",
				zap.Uint64("height", height))
		}

		logger.Info("backfilled momentum",
			zap.Uint64("height", height),
			zap.Int("txCount", len(momentums.List[0].Content)))
	}

	return nil
}
