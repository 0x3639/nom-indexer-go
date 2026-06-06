//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_Htlc_InsertSettleList(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewHtlcRepository(pool)

	h := &models.Htlc{
		ID:                        "a1",
		TimeLockedAddress:         "z1qsender",
		HashLockedAddress:         "z1qreceiver",
		TokenStandard:             models.ZnnTokenStandard,
		Amount:                    500,
		ExpirationTimestamp:       1700001000,
		HashType:                  0,
		KeyMaxSize:                32,
		HashLock:                  "deadbeef",
		Status:                    int16(models.HtlcStatusActive),
		CreationMomentumHeight:    10,
		CreationMomentumTimestamp: 1700000000,
	}
	if err := repo.Insert(ctx, h); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.Insert(ctx, h); err != nil {
		t.Fatalf("re-insert: %v", err)
	}
	if err := repo.Settle(ctx, "a1", int16(models.HtlcStatusUnlocked), "c0ffee", 11, 1700000100); err != nil {
		t.Fatalf("settle: %v", err)
	}
	got, err := repo.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != int16(models.HtlcStatusUnlocked) || got.Preimage != "c0ffee" || got.SettleMomentumHeight != 11 {
		t.Errorf("settle not applied: %+v", got)
	}
	list, total, err := repo.List(ctx, ListOpts{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("list total=%d len=%d, want 1/1", total, len(list))
	}
}

func TestIntegration_Htlc_BatchAndPagination(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewHtlcRepository(pool)

	low := &models.Htlc{
		ID:                        "b10",
		TimeLockedAddress:         "z1qsender",
		HashLockedAddress:         "z1qreceiver",
		TokenStandard:             models.ZnnTokenStandard,
		Amount:                    100,
		ExpirationTimestamp:       1700001000,
		HashType:                  0,
		KeyMaxSize:                32,
		HashLock:                  "aa",
		Status:                    int16(models.HtlcStatusActive),
		CreationMomentumHeight:    10,
		CreationMomentumTimestamp: 1700000000,
	}
	high := &models.Htlc{
		ID:                        "b20",
		TimeLockedAddress:         "z1qsender",
		HashLockedAddress:         "z1qreceiver",
		TokenStandard:             models.ZnnTokenStandard,
		Amount:                    200,
		ExpirationTimestamp:       1700002000,
		HashType:                  0,
		KeyMaxSize:                32,
		HashLock:                  "bb",
		Status:                    int16(models.HtlcStatusActive),
		CreationMomentumHeight:    20,
		CreationMomentumTimestamp: 1700000200,
	}

	batch := &pgx.Batch{}
	repo.InsertBatch(batch, low)
	repo.InsertBatch(batch, high)
	repo.SettleBatch(batch, "b20", int16(models.HtlcStatusUnlocked), "c0ffee", 21, 1700000300)
	sendBatch(t, ctx, pool, batch)

	gotLow, err := repo.GetByID(ctx, "b10")
	if err != nil {
		t.Fatalf("get b10: %v", err)
	}
	if gotLow.Status != int16(models.HtlcStatusActive) || gotLow.Amount != 100 {
		t.Errorf("b10 not inserted as active: %+v", gotLow)
	}
	gotHigh, err := repo.GetByID(ctx, "b20")
	if err != nil {
		t.Fatalf("get b20: %v", err)
	}
	if gotHigh.Status != int16(models.HtlcStatusUnlocked) || gotHigh.Preimage != "c0ffee" || gotHigh.SettleMomentumHeight != 21 {
		t.Errorf("b20 settle not applied: %+v", gotHigh)
	}

	// Page 0: DESC order returns the height-20 row first; COUNT(*) OVER() == 2.
	page0, total, err := repo.List(ctx, ListOpts{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("list page0: %v", err)
	}
	if total != 2 || len(page0) != 1 || page0[0].ID != "b20" {
		t.Errorf("page0 total=%d rows=%d first=%v, want total=2 first=b20", total, len(page0), idOrEmpty(page0))
	}

	// Page 1: drives the COUNT(*) OVER() path with a non-empty page at offset 1.
	page1, total, err := repo.List(ctx, ListOpts{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if total != 2 || len(page1) != 1 || page1[0].ID != "b10" {
		t.Errorf("page1 total=%d rows=%d first=%v, want total=2 first=b10", total, len(page1), idOrEmpty(page1))
	}

	// Offset beyond the result set: empty page drives the fallbackCount branch.
	empty, total, err := repo.List(ctx, ListOpts{Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if total != 2 || len(empty) != 0 {
		t.Errorf("empty page total=%d rows=%d, want total=2 rows=0", total, len(empty))
	}
}

func idOrEmpty(hs []*models.Htlc) string {
	if len(hs) == 0 {
		return ""
	}
	return hs[0].ID
}
