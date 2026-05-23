// Package repository is the data-access layer: one file per table, each
// exporting a Repository type with CRUD + batched-variant methods.
//
// Batched variants take a *pgx.Batch and queue the SQL without sending
// it; the caller (typically processMomentum) sends the whole batch in
// one transaction. The pattern lets every write for a single momentum
// share rollback semantics.
//
// Inserts use ON CONFLICT … DO NOTHING for event-keyed tables and
// ON CONFLICT … DO UPDATE for state tables. Additive counters
// (cumulative_rewards) need extra care — see
// docs/schema/conventions.md#batch-writes-and-idempotency.
package repository
