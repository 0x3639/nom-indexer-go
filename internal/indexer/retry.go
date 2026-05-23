package indexer

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// retryConfig controls withRetry. Defaults give ~32s of total wait before
// giving up: 0.5s + 1s + 2s + 4s + 8s + 16s.
type retryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxBackoff  time.Duration
}

func defaultRetry() retryConfig {
	return retryConfig{
		MaxAttempts: 6,
		BaseDelay:   500 * time.Millisecond,
		MaxBackoff:  30 * time.Second,
	}
}

// withRetry runs fn with exponential backoff until it succeeds, ctx is
// cancelled, or maxAttempts is reached. Transient RPC/DB errors should not
// kill the sync loop; persistent ones should bubble up after enough attempts.
//
// The label is included in retry log lines and in the final wrapped error so
// the caller's failure site is easy to spot.
func withRetry(ctx context.Context, logger *zap.Logger, label string, fn func() error) error {
	return withRetryConfig(ctx, logger, label, defaultRetry(), fn)
}

func withRetryConfig(ctx context.Context, logger *zap.Logger, label string, cfg retryConfig, fn func() error) error {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	delay := cfg.BaseDelay

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		logger.Warn("transient error, retrying",
			zap.String("op", label),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", cfg.MaxAttempts),
			zap.Duration("backoff", delay),
			zap.Error(lastErr))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		if delay < cfg.MaxBackoff {
			delay *= 2
			if delay > cfg.MaxBackoff {
				delay = cfg.MaxBackoff
			}
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", label, cfg.MaxAttempts, lastErr)
}
