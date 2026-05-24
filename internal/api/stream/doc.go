// Package stream owns the in-process pub/sub hub that fans newly-indexed
// momentums out to WebSocket subscribers.
//
// # Architecture
//
// Exactly one Hub runs per API process. It holds a single dedicated
// pgx.Conn (outside the request pool) that issues `LISTEN momentum_new`
// once at startup and then blocks on WaitForNotification in a goroutine
// for the process lifetime. Postgres fires the NOTIFY from the indexer's
// processMomentum after every successful commit (see
// internal/indexer/processor.go).
//
// DB cost
//
//   - One persistent connection per API process, doing nothing 99% of
//     the time (LISTEN is server-pushed).
//   - Zero queries on the steady-state path; NOTIFY is cheap for the
//     indexer too — Postgres' pg_notify is a no-op when no listener is
//     attached, so the indexer pays nothing when no streams are open.
//   - One catch-up SELECT per client reconnect that requests
//     ?from_height=N.
//
// # Fan-out
//
// Subscribers register a buffered channel via Subscribe. The dispatch
// goroutine writes to every subscriber's channel non-blockingly; if the
// channel is full (client is slow), the message is dropped and a
// per-subscriber `lagged` counter increments. The handler is expected
// to close the connection when it sees lag — we don't try to recover
// it inline.
//
// # Why not polling
//
// At the rate momentums commit (one every ~10s on mainnet), polling
// would issue ~8,640 queries/day per API process with zero benefit
// over NOTIFY's instant push. Polling also can't deliver sub-second
// latency without burning hot CPU. LISTEN is strictly cheaper and
// strictly faster — at the cost of one indexer-side change
// (notifyMomentum) that's already in this branch.
package stream
