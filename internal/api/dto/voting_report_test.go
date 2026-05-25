package dto

import (
	"reflect"
	"testing"
)

func TestVoteCodeToString(t *testing.T) {
	t.Parallel()
	cases := map[int16]string{0: "yes", 1: "no", 2: "abstain", 3: "unknown", 99: "unknown", -1: "unknown"}
	for code, want := range cases {
		if got := VoteCodeToString(code); got != want {
			t.Errorf("VoteCodeToString(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestFromProposalTally_ExpandsAndSorts(t *testing.T) {
	t.Parallel()
	got := FromProposalTally(RawProposalTally{
		VotingID: "voting-xyz",
		ByPillar: map[string]int16{
			"Charlie": 0,  // yes
			"Alice":   0,  // yes
			"Diana":   1,  // no
			"Bob":     2,  // abstain
			"Eve":     99, // unknown — should be dropped
		},
		NoVotePillars: []string{"Zelda", "Mallory"},
	})

	want := ProposalVoteTally{
		VotingID:       "voting-xyz",
		YesCount:       2,
		NoCount:        1,
		AbstainCount:   1,
		NoVoteCount:    2,
		YesPillars:     []string{"Alice", "Charlie"},
		NoPillars:      []string{"Diana"},
		AbstainPillars: []string{"Bob"},
		NoVotePillars:  []string{"Mallory", "Zelda"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromProposalTally mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestFromPillarVotingHistory_TalliesAgreeWithRows(t *testing.T) {
	t.Parallel()
	raws := []RawPillarVote{
		{VotingID: "a", Vote: 0, ProjectID: "p1", ProjectName: "Alpha", MomentumHeight: 100, MomentumTimestamp: 1000},
		{VotingID: "b", Vote: 1, ProjectID: "p2", ProjectName: "Beta", PhaseID: "ph1", PhaseName: "Phase 1", MomentumHeight: 200, MomentumTimestamp: 2000},
		{VotingID: "c", Vote: 2, ProjectID: "p1", ProjectName: "Alpha", PhaseID: "ph2", PhaseName: "Phase 2", MomentumHeight: 300, MomentumTimestamp: 3000},
		{VotingID: "d", Vote: 0, ProjectID: "p3", ProjectName: "Gamma", MomentumHeight: 400, MomentumTimestamp: 4000},
	}
	got := FromPillarVotingHistory("MyPillar", "z1qowner", raws)

	if got.PillarName != "MyPillar" || got.PillarOwner != "z1qowner" {
		t.Errorf("identity mismatch: %+v", got)
	}
	if got.TotalVotes != 4 || got.YesCount != 2 || got.NoCount != 1 || got.AbstainCount != 1 {
		t.Errorf("tallies mismatch: total=%d yes=%d no=%d abstain=%d", got.TotalVotes, got.YesCount, got.NoCount, got.AbstainCount)
	}
	if len(got.Votes) != 4 {
		t.Fatalf("expected 4 vote entries, got %d", len(got.Votes))
	}
	// Spot-check the first translated entry preserves all fields + translates the code.
	if got.Votes[0].Vote != "yes" || got.Votes[0].ProjectName != "Alpha" || got.Votes[0].MomentumHeight != 100 {
		t.Errorf("first entry not translated correctly: %+v", got.Votes[0])
	}
	// Mid-row with phase fields populated.
	if got.Votes[1].Vote != "no" || got.Votes[1].PhaseName != "Phase 1" {
		t.Errorf("second entry phase fields lost: %+v", got.Votes[1])
	}
}
