package indexer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWithRetry_SucceedsImmediately(t *testing.T) {
	calls := 0
	err := withRetry(context.Background(), zap.NewNop(), "op", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_SucceedsAfterTransient(t *testing.T) {
	calls := 0
	cfg := retryConfig{MaxAttempts: 5, BaseDelay: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond}

	err := withRetryConfig(context.Background(), zap.NewNop(), "op", cfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	cfg := retryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxBackoff: 5 * time.Millisecond}

	err := withRetryConfig(context.Background(), zap.NewNop(), "myop", cfg, func() error {
		calls++
		return errors.New("persistent")
	})
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	if !strings.Contains(err.Error(), "myop") || !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("error should mention label and attempts, got %q", err.Error())
	}
}

func TestWithRetry_ContextCancelStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cfg := retryConfig{MaxAttempts: 10, BaseDelay: 50 * time.Millisecond, MaxBackoff: 50 * time.Millisecond}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := withRetryConfig(ctx, zap.NewNop(), "op", cfg, func() error {
		calls++
		return errors.New("transient")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls > 2 {
		t.Errorf("expected at most a couple of calls before cancel, got %d", calls)
	}
}

func TestWithRetry_PreCancelledContextRetursnImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := withRetry(ctx, zap.NewNop(), "op", func() error {
		calls++
		return errors.New("never reached")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls before any work, got %d", calls)
	}
}
