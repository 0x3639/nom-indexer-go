//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_BridgeTimeChallenge_UpsertAndPrune(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBridgeConfigRepository(pool)

	mustUpsert := func(name string) {
		if err := repo.UpsertTimeChallenge(ctx, &models.BridgeTimeChallenge{
			MethodName: name, ParamsHash: "h", ChallengeStartHeight: 100, Delay: 10, EndHeight: 110,
			LastUpdatedTimestamp: 1,
		}); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}
	mustUpsert("ChangeAdministrator")
	mustUpsert("NominateGuardians")

	if err := repo.DeleteTimeChallengesNotIn(ctx, []string{"ChangeAdministrator"}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	list, err := repo.ListTimeChallenges(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].MethodName != "ChangeAdministrator" {
		t.Errorf("prune wrong: %+v", list)
	}

	if err := repo.DeleteTimeChallengesNotIn(ctx, nil); err != nil {
		t.Fatalf("prune all: %v", err)
	}
	list, _ = repo.ListTimeChallenges(ctx)
	if len(list) != 0 {
		t.Errorf("expected empty after prune-all, got %+v", list)
	}
}
