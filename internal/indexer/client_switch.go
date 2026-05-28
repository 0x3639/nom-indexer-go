package indexer

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// swapClient verifies that the candidate node is on the expected chain,
// then publishes a new client via publish() and signals the subscription
// to restart via restart(). The publish/restart callbacks are dependency-
// injected so this function is unit-testable without an Indexer.
//
// storedGenesis is the canonical chain identifier (genesis momentum hash)
// previously recorded in indexer_sync_status. Pass "" on the first-ever
// run to skip the comparison and accept whatever genesis the candidate
// reports — the watchdog will then store it as the canonical value.
func swapClient(
	ctx context.Context,
	logger *zap.Logger,
	pool *NodePool,
	candidateIdx int,
	storedGenesis string,
	publish func(newURL string) error,
	restart func(),
) error {
	if candidateIdx < 0 || candidateIdx >= pool.Len() {
		return fmt.Errorf("swapClient: idx %d out of range (len=%d)", candidateIdx, pool.Len())
	}
	entry := pool.Entry(candidateIdx)

	result, err := pool.Probe(ctx, candidateIdx)
	if err != nil {
		return fmt.Errorf("probe candidate %q: %w", entry.Label, err)
	}
	if storedGenesis != "" && result.GenesisHash != storedGenesis {
		return fmt.Errorf("chain mismatch for %q: candidate genesis=%q, stored=%q",
			entry.Label, result.GenesisHash, storedGenesis)
	}

	if err := publish(entry.URL); err != nil {
		return fmt.Errorf("publish new client: %w", err)
	}
	restart()

	logger.Info("swapped active client",
		zap.String("label", entry.Label),
		zap.String("url", entry.URL))
	return nil
}
