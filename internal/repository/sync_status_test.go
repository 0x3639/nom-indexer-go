//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/jackc/pgx/v5"
)

func TestSyncStatusUpsertGet(t *testing.T) {
	ctx := context.Background()
	pool := newTestDB(t)
	repo := NewSyncStatusRepository(pool)

	want := &models.SyncStatus{
		DBHeight:           100,
		ZnndFrontierHeight: 100,
		ZnndTargetHeight:   100,
		State:              "synced",
		ActiveNodeURL:      "ws://znnd:35998",
		ActiveNodeLabel:    "local",
		ChainIdentifier:    "genesis-hash",
		LastProgressAt:     1000,
		CheckedAt:          1001,
	}
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DBHeight != want.DBHeight || got.State != want.State ||
		got.ZnndFrontierHeight != want.ZnndFrontierHeight ||
		got.ZnndTargetHeight != want.ZnndTargetHeight ||
		got.ActiveNodeURL != want.ActiveNodeURL ||
		got.ActiveNodeLabel != want.ActiveNodeLabel ||
		got.ChainIdentifier != want.ChainIdentifier ||
		got.LastProgressAt != want.LastProgressAt ||
		got.CheckedAt != want.CheckedAt {
		t.Fatalf("field mismatch:\n got %+v\n want %+v", got, want)
	}
	if got.FailedOverAt != nil {
		t.Fatalf("expected nil FailedOverAt, got %d", *got.FailedOverAt)
	}
}

func TestSyncStatusUpsertOverwrites(t *testing.T) {
	ctx := context.Background()
	pool := newTestDB(t)
	repo := NewSyncStatusRepository(pool)

	s1 := &models.SyncStatus{State: "synced", ActiveNodeURL: "u1", ActiveNodeLabel: "l1", ChainIdentifier: "g"}
	if err := repo.Upsert(ctx, s1); err != nil {
		t.Fatal(err)
	}

	s2 := &models.SyncStatus{State: "node_lagging", ActiveNodeURL: "u2", ActiveNodeLabel: "l2", ChainIdentifier: "g", DBHeight: 99}
	if err := repo.Upsert(ctx, s2); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "node_lagging" || got.ActiveNodeLabel != "l2" || got.DBHeight != 99 {
		t.Fatalf("expected second upsert to overwrite, got %+v", got)
	}
}

func TestSyncStatusSingletonConstraint(t *testing.T) {
	ctx := context.Background()
	pool := newTestDB(t)

	_, err := pool.Exec(ctx,
		`INSERT INTO indexer_sync_status (id, db_height, znnd_frontier_height, znnd_target_height,
         drift_momentums, node_lag_momentums, state, active_node_url, active_node_label,
         chain_identifier, last_progress_at, checked_at)
         VALUES (2, 0, 0, 0, 0, 0, 'synced', '', '', '', 0, 0)`)
	if err == nil {
		t.Fatal("expected CHECK (id = 1) to reject id=2 insert")
	}
}

func TestSyncStatusGetEmptyTable(t *testing.T) {
	ctx := context.Background()
	pool := newTestDB(t)
	repo := NewSyncStatusRepository(pool)
	_, err := repo.Get(ctx)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}
