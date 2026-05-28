package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/0x3639/nom-indexer-go/internal/database"
	"github.com/0x3639/nom-indexer-go/internal/health"
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

	if len(cfg.Indexer.Nodes) == 0 {
		logger.Fatal("no nodes configured (set NODE_URL_WS or indexer.nodes)")
	}

	primaryURL := cfg.Indexer.Nodes[0].URL

	logger.Info("starting nom-indexer",
		zap.String("node_url", primaryURL),
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

	// Connect to primary Zenon node. The watchdog (if enabled) may swap
	// this out at runtime via swapActiveClient.
	client, err := rpc_client.NewRpcClient(primaryURL)
	if err != nil {
		logger.Fatal("failed to connect to primary node", zap.Error(err))
	}
	defer client.Stop()

	logger.Info("connected to Zenon node",
		zap.String("url", primaryURL),
		zap.String("label", cfg.Indexer.Nodes[0].Label),
	)

	votingInterval, err := indexer.ParseCronInterval(cfg.Cron.VotingActivityInterval, 10*time.Minute)
	if err != nil {
		logger.Fatal("invalid cron.voting_activity_interval", zap.Error(err))
	}
	tokenHoldersInterval, err := indexer.ParseCronInterval(cfg.Cron.TokenHoldersInterval, 10*time.Minute)
	if err != nil {
		logger.Fatal("invalid cron.token_holders_interval", zap.Error(err))
	}

	nodePool := indexer.NewNodePool(toIndexerNodes(cfg.Indexer.Nodes), logger)

	idx := indexer.NewIndexerWithNodes(pool, nodePool, client, logger,
		indexer.CronConfig{
			VotingActivityInterval: votingInterval,
			TokenHoldersInterval:   tokenHoldersInterval,
		},
		indexer.WatchdogConfigForIndexer{
			Enabled:               cfg.Indexer.Watchdog.Enabled,
			Interval:              cfg.Indexer.Watchdog.Interval,
			StallThreshold:        cfg.Indexer.Watchdog.StallThreshold,
			IndexerDriftThreshold: cfg.Indexer.Watchdog.IndexerDriftThreshold,
			NodeDriftThreshold:    cfg.Indexer.Watchdog.NodeDriftThreshold,
			UnhealthyStreak:       cfg.Indexer.Watchdog.UnhealthyStreak,
			FailbackStreak:        cfg.Indexer.Watchdog.FailbackStreak,
		},
	)

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

	// Synchronous startup probe. If the watchdog is enabled and the primary
	// fails, walk down the configured fallbacks until one is reachable.
	if cfg.Indexer.Watchdog.Enabled {
		probe, err := nodePool.Probe(ctx, 0)
		if err != nil {
			logger.Warn("startup probe of primary failed, trying fallbacks", zap.Error(err))
			swapped := false
			for i := 1; i < nodePool.Len(); i++ {
				if _, err := nodePool.Probe(ctx, i); err == nil {
					if err := idx.StartupSwap(i); err != nil {
						logger.Error("startup swap failed", zap.Int("idx", i), zap.Error(err))
						continue
					}
					logger.Info("started on fallback node", zap.String("label", nodePool.Entry(i).Label))
					swapped = true
					break
				}
			}
			if !swapped {
				logger.Fatal("all nodes failed startup probe")
			}
		} else {
			logger.Info("startup probe ok",
				zap.Uint64("frontier", probe.Frontier),
				zap.String("genesis", probe.GenesisHash),
			)
		}
	}

	// Launch the indexer's HTTP health server. Started before backfill so
	// /healthz answers during the (potentially slow) backfill phase too.
	if cfg.Indexer.Health.Enabled {
		healthSrv := health.NewServer(func() health.Snapshot {
			s := idx.HealthSnapshot()
			return health.Snapshot{
				Ready:     s.Ready,
				State:     s.State,
				NodeLabel: s.NodeLabel,
				Drift:     s.Drift,
			}
		})
		go func() {
			addr := fmt.Sprintf(":%d", cfg.Indexer.Health.Port)
			logger.Info("starting health server", zap.String("addr", addr))
			if err := healthSrv.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
				logger.Error("health server crashed", zap.Error(err))
			}
		}()
	}

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

// toIndexerNodes adapts the config-level NodeEntry list to the
// indexer-package NodeEntry list. Both have the same shape; we keep
// them separate so internal/indexer doesn't import internal/config.
func toIndexerNodes(in []config.NodeEntry) []indexer.NodeEntry {
	out := make([]indexer.NodeEntry, len(in))
	for i, n := range in {
		out[i] = indexer.NodeEntry{URL: n.URL, Label: n.Label, ProbeURL: n.ProbeURL}
	}
	return out
}
