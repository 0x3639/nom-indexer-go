package repository

import (
	"reflect"
	"testing"
)

func TestProposalTallyRow_SetNoVote_DiffsAgainstActiveSet(t *testing.T) {
	t.Parallel()
	t.Run("populates from full active set", func(t *testing.T) {
		row := ProposalTallyRow{
			ByPillar: map[string]int16{"Alice": 0, "Bob": 1},
		}
		row.SetNoVote([]string{"Alice", "Bob", "Charlie", "Diana"})
		want := []string{"Charlie", "Diana"}
		if !reflect.DeepEqual(row.NoVotePillars, want) {
			t.Errorf("NoVotePillars = %v, want %v", row.NoVotePillars, want)
		}
	})
	t.Run("empty when everyone voted", func(t *testing.T) {
		row := ProposalTallyRow{
			ByPillar: map[string]int16{"Alice": 0, "Bob": 1},
		}
		row.SetNoVote([]string{"Alice", "Bob"})
		if len(row.NoVotePillars) != 0 {
			t.Errorf("NoVotePillars should be empty; got %v", row.NoVotePillars)
		}
	})
	t.Run("active pillar order is preserved", func(t *testing.T) {
		row := ProposalTallyRow{ByPillar: map[string]int16{}}
		row.SetNoVote([]string{"Zelda", "Alice", "Mallory"})
		want := []string{"Zelda", "Alice", "Mallory"}
		if !reflect.DeepEqual(row.NoVotePillars, want) {
			t.Errorf("order = %v, want %v (SetNoVote should not re-sort; caller sorts upstream)", row.NoVotePillars, want)
		}
	})
}
