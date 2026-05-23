// Package indexer is the heart of nom-indexer-go: the per-momentum
// processing pipeline, the sync + subscription loop, the bridge sync,
// and the cron jobs.
//
// Indexer.Run coordinates these long-lived lanes:
//
//   - main sync / subscription in the foreground (reactive)
//   - bridge sync goroutine (1 minute cadence)
//   - cached data sync goroutine (5 minute cadence — pillars, sentinels, projects)
//   - cron loop goroutine (10 min / 1 h — voting activity, holder counts, daily snapshots)
//   - the SDK's own connection lifecycle
//
// Per-momentum processing is transactional: every write for a single
// momentum lands in one pgx.Batch wrapped in a transaction. A failure
// rolls back and the sync loop retries the height.
//
// See docs/architecture/overview.md for the system picture and
// docs/architecture/data-flow.md for the per-momentum trace.
package indexer
