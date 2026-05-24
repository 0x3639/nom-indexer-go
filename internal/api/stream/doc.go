// Package stream owns the in-process pub/sub hub that fans
// newly-indexed events out to WebSocket subscribers.
//
// # Architecture
//
// Hub is generic over the dispatched type T (today: *dto.Momentum for
// the momentums stream, *dto.AccountBlock for the transactions
// stream). Each Hub instance holds a single dedicated pgx.Conn
// (outside the request pool) that issues `LISTEN <channel>` for each
// connection attempt, blocks on WaitForNotification, and reconnects
// with backoff if the LISTEN connection drops.
//
// The indexer's processMomentum queues NOTIFY statements
// (`momentum_new`, `account_block_new`) in the same transaction as
// the row writes, so Postgres fires them only after a successful
// commit — see internal/indexer/processor.go.
//
// One Hub per (channel, type) per API process. cmd/api wires up one
// MomentumHub and one TxHub and runs both as background goroutines.
//
// # DB cost
//
//   - One persistent connection per hub per API process, doing nothing
//     99% of the time (LISTEN is server-pushed).
//   - Zero queries on the steady-state path; NOTIFY is cheap for the
//     indexer too — Postgres' pg_notify is a no-op when no listener is
//     attached, so the indexer pays nothing when no streams are open.
//   - One catch-up SELECT per client reconnect that requests
//     ?from_height=N.
//
// # Fan-out
//
// Subscribers register a buffered channel via Subscribe. The dispatch
// goroutine writes to every subscriber's channel non-blockingly; if
// the channel is full (client is slow), the message is dropped and a
// per-subscriber `lagged` counter increments. The handler is expected
// to close the connection when it sees lag — we don't try to recover
// it inline.
//
// # Filtering
//
// The hub fan-outs are unfiltered. Per-connection filters (e.g. the
// transactions stream's ?address= query parameter) are implemented in
// the handler's live loop, BEFORE writing to the WS — keeping the hub
// uniform and pushing client-specific predicates to where the
// connection context exists.
//
// # Why not polling
//
// At the rate momentums commit (one every ~10s on mainnet), polling
// would issue ~8,640 queries/day per API process with zero benefit
// over NOTIFY's instant push. Polling also can't deliver sub-second
// latency without burning hot CPU. LISTEN is strictly cheaper and
// strictly faster — at the cost of one indexer-side change (the
// queueMomentumNotify / queueAccountBlockNotify calls in
// processMomentum) that's now part of the standard write path.
package stream
