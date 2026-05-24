//go:build integration

package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_VotingReports_ResolveOwnerAndProducerVotes(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()

	pillars := NewPillarRepository(pool)
	pillarUpdates := NewPillarUpdateRepository(pool)
	projects := NewProjectRepository(pool)
	phases := NewProjectPhaseRepository(pool)
	votes := NewVoteRepository(pool)

	for _, p := range []*models.Pillar{
		{OwnerAddress: "z1qalpha-owner", ProducerAddress: "z1qalpha-current-prod", WithdrawAddress: "z1qalpha-withdraw", Name: "Alpha", Rank: 1},
		{OwnerAddress: "z1qbeta-owner", ProducerAddress: "z1qbeta-current-prod", WithdrawAddress: "z1qbeta-withdraw", Name: "Beta", Rank: 2},
		{OwnerAddress: "z1qgamma-owner", ProducerAddress: "z1qgamma-current-prod", WithdrawAddress: "z1qgamma-withdraw", Name: "Gamma", Rank: 3},
	} {
		if err := pillars.Upsert(ctx, p); err != nil {
			t.Fatalf("pillar upsert %s: %v", p.Name, err)
		}
	}
	if err := pillarUpdates.Insert(ctx, &models.PillarUpdate{
		Name: "Alpha", OwnerAddress: "z1qalpha-owner", ProducerAddress: "z1qalpha-old-prod",
		WithdrawAddress: "z1qalpha-withdraw", MomentumHeight: 1, MomentumTimestamp: 100,
		MomentumHash: "mh-alpha-register",
	}); err != nil {
		t.Fatalf("pillar update insert: %v", err)
	}
	if err := projects.Upsert(ctx, &models.Project{
		ID: "p1", VotingID: "v-project", Owner: "z1qproject-owner", Name: "Project One", CreationTimestamp: 1000,
	}); err != nil {
		t.Fatalf("project upsert: %v", err)
	}
	if err := phases.Upsert(ctx, &models.ProjectPhase{
		ID: "ph1", ProjectID: "p1", VotingID: "v-phase", Name: "Phase One", CreationTimestamp: 1100,
	}); err != nil {
		t.Fatalf("phase upsert: %v", err)
	}

	for _, v := range []*models.Vote{
		{MomentumHash: "mh-alpha-owner", MomentumHeight: 10, MomentumTimestamp: 1000, VoterAddress: "z1qalpha-owner", ProjectID: "p1", VotingID: "v-project", Vote: 0},
		{MomentumHash: "mh-beta-prod", MomentumHeight: 11, MomentumTimestamp: 1100, VoterAddress: "z1qbeta-current-prod", ProjectID: "p1", VotingID: "v-project", Vote: 1},
		{MomentumHash: "mh-alpha-old-prod", MomentumHeight: 20, MomentumTimestamp: 2000, VoterAddress: "z1qalpha-old-prod", ProjectID: "p1", PhaseID: "ph1", VotingID: "v-phase", Vote: 2},
	} {
		if err := votes.Insert(ctx, v); err != nil {
			t.Fatalf("vote insert %s: %v", v.MomentumHash, err)
		}
	}

	report, err := votes.ProjectVotingReport(ctx, "p1")
	if err != nil {
		t.Fatalf("ProjectVotingReport: %v", err)
	}
	if report.ActivePillarCount != 3 {
		t.Fatalf("ActivePillarCount = %d, want 3", report.ActivePillarCount)
	}
	if got := report.Project.ByPillar; !reflect.DeepEqual(got, map[string]int16{"Alpha": 0, "Beta": 1}) {
		t.Fatalf("project ByPillar = %v, want Alpha yes + Beta no", got)
	}
	if got := report.Project.NoVotePillars; !reflect.DeepEqual(got, []string{"Gamma"}) {
		t.Fatalf("project NoVotePillars = %v, want [Gamma]", got)
	}
	if len(report.Phases) != 1 {
		t.Fatalf("phase count = %d, want 1", len(report.Phases))
	}
	if got := report.Phases[0].Tally.ByPillar; !reflect.DeepEqual(got, map[string]int16{"Alpha": 2}) {
		t.Fatalf("phase ByPillar = %v, want Alpha abstain via historical producer", got)
	}
	if got := report.Phases[0].Tally.NoVotePillars; !reflect.DeepEqual(got, []string{"Beta", "Gamma"}) {
		t.Fatalf("phase NoVotePillars = %v, want [Beta Gamma]", got)
	}

	alphaHistory, err := votes.PillarVotingHistory(ctx, "Alpha")
	if err != nil {
		t.Fatalf("PillarVotingHistory Alpha: %v", err)
	}
	if len(alphaHistory.Votes) != 2 {
		t.Fatalf("Alpha history count = %d, want 2", len(alphaHistory.Votes))
	}
	if alphaHistory.Votes[0].VotingID != "v-phase" || alphaHistory.Votes[1].VotingID != "v-project" {
		t.Fatalf("Alpha history order/voting IDs = %+v", alphaHistory.Votes)
	}

	betaHistory, err := votes.PillarVotingHistory(ctx, "Beta")
	if err != nil {
		t.Fatalf("PillarVotingHistory Beta: %v", err)
	}
	if len(betaHistory.Votes) != 1 || betaHistory.Votes[0].VotingID != "v-project" {
		t.Fatalf("Beta history = %+v, want current-producer vote on v-project", betaHistory.Votes)
	}
}
