//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

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

func TestIntegration_Swap_BatchAndPagination(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewSwapRepository(pool)

	low := &models.SwapRetrieval{
		ID: "r10", Address: "z1qclaim", PublicKey: "pk", ZnnAmount: 100, QsrAmount: 200,
		MomentumHeight: 10, MomentumTimestamp: 1700000000,
	}
	high := &models.SwapRetrieval{
		ID: "r20", Address: "z1qclaim", PublicKey: "pk", ZnnAmount: 300, QsrAmount: 400,
		MomentumHeight: 20, MomentumTimestamp: 1700000200,
	}

	batch := &pgx.Batch{}
	repo.InsertRetrievalBatch(batch, low)
	repo.InsertRetrievalBatch(batch, high)
	sendBatch(t, ctx, pool, batch)

	// Both rows present: COUNT(*) OVER() == 2.
	all, total, err := repo.ListRetrievals(ctx, ListOpts{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Errorf("list all total=%d rows=%d, want total=2 rows=2", total, len(all))
	}

	// Page 0: DESC order returns the height-20 row first.
	page0, total, err := repo.ListRetrievals(ctx, ListOpts{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("list page0: %v", err)
	}
	if total != 2 || len(page0) != 1 || page0[0].ID != "r20" {
		t.Errorf("page0 total=%d rows=%d first=%v, want total=2 first=r20", total, len(page0), retrievalIDOrEmpty(page0))
	}

	// Page 1: drives the COUNT(*) OVER() path with a non-empty page at offset 1.
	page1, total, err := repo.ListRetrievals(ctx, ListOpts{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if total != 2 || len(page1) != 1 || page1[0].ID != "r10" {
		t.Errorf("page1 total=%d rows=%d first=%v, want total=2 first=r10", total, len(page1), retrievalIDOrEmpty(page1))
	}

	// Offset beyond the result set: empty page drives the fallbackCount branch.
	empty, total, err := repo.ListRetrievals(ctx, ListOpts{Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if total != 2 || len(empty) != 0 {
		t.Errorf("empty page total=%d rows=%d, want total=2 rows=0", total, len(empty))
	}
}

func retrievalIDOrEmpty(rs []*models.SwapRetrieval) string {
	if len(rs) == 0 {
		return ""
	}
	return rs[0].ID
}
