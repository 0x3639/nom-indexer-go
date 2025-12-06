package database

import (
	"context"
	"fmt"
	"time"

	"github.com/0x3639/nom-indexer-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewPool creates a new PostgreSQL connection pool
func NewPool(ctx context.Context, cfg *config.DatabaseConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	connString := cfg.ConnectionString()

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = int32(cfg.PoolSize)
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	logger.Info("connecting to database",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
		zap.Int("pool_size", cfg.PoolSize))

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("connected to database successfully")

	return pool, nil
}

// HealthCheck performs a health check on the database connection
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return pool.Ping(ctx)
}
