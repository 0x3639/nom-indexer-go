//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_Swap_RetrievalAndAssetUpsert(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewSwapRepository(pool)

	if err := repo.InsertRetrieval(ctx, &models.SwapRetrieval{
		ID: "r1", Address: "z1qclaim", PublicKey: "pk", ZnnAmount: 100, QsrAmount: 200,
		MomentumHeight: 5, MomentumTimestamp: 1700000000,
	}); err != nil {
		t.Fatalf("insert retrieval: %v", err)
	}
	if err := repo.UpsertAsset(ctx, &models.SwapAsset{KeyIDHash: "k1", Znn: 10, Qsr: 20, LastUpdatedTimestamp: 1}); err != nil {
		t.Fatalf("upsert asset 1: %v", err)
	}
	if err := repo.UpsertAsset(ctx, &models.SwapAsset{KeyIDHash: "k1", Znn: 7, Qsr: 0, LastUpdatedTimestamp: 2}); err != nil {
		t.Fatalf("upsert asset 2: %v", err)
	}
	a, err := repo.GetAsset(ctx, "k1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if a.Znn != 7 || a.Qsr != 0 || a.LastUpdatedTimestamp != 2 {
		t.Errorf("asset not upserted: %+v", a)
	}
}
