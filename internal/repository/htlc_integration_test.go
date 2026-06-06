//go:build integration

package repository

import (
	"context"
	"testing"

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
