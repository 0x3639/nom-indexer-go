package dto

import "sort"

// ProjectVotingReport is a complete server-side report of how every
// currently-active pillar voted on a single Accelerator-Z project AND
// all of its phases. One repository call, one HTTP/MCP call, no
// client-side joining over pillars × proposals.
//
// Layout: Project carries the tally on the project's own proposal;
// Phases is one entry per phase (in creation order) carrying its own
// tally. ActivePillarCount is the denominator used for each tally —
// pillars listed in NoVotePillars are active pillars that have not
// (yet) voted on that specific proposal.
//
// Pillar lists are slim (names only) so the report stays compact even
// when the active-pillar set is 70+ deep.
type ProjectVotingReport struct {
	ProjectID         string            `json:"project_id"`
	ProjectName       string            `json:"project_name"`
	ActivePillarCount int               `json:"active_pillar_count"`
	Project           ProposalVoteTally `json:"project"`
	Phases            []PhaseVoteTally  `json:"phases"`
}

// ProposalVoteTally is the per-proposal breakdown shared by the
// project-level entry and every phase entry. yes/no/abstain counts +
// the explicit list of which pillars voted which way; NoVotePillars
// holds active pillars who never voted (i.e. abstained by omission).
type ProposalVoteTally struct {
	VotingID       string   `json:"voting_id"`
	YesCount       int      `json:"yes_count"`
	NoCount        int      `json:"no_count"`
	AbstainCount   int      `json:"abstain_count"`
	NoVoteCount    int      `json:"no_vote_count"`
	YesPillars     []string `json:"yes_pillars"`
	NoPillars      []string `json:"no_pillars"`
	AbstainPillars []string `json:"abstain_pillars"`
	NoVotePillars  []string `json:"no_vote_pillars"`
}

// PhaseVoteTally wraps ProposalVoteTally with the phase identity. Phases
// are ordered by creation_timestamp ASC (phase 1 first), matching the
// docs/schema/project_phases.md convention.
type PhaseVoteTally struct {
	PhaseID   string            `json:"phase_id"`
	PhaseName string            `json:"phase_name"`
	Vote      ProposalVoteTally `json:"vote"`
}

// PillarVotingHistory is the complete voting record of one named pillar
// across every project + phase, with project + phase names already
// resolved server-side. Ordered by momentum_timestamp DESC (newest
// vote first).
//
// Totals are derived from len(Votes) and the per-row vote field, so
// they always agree with the rows.
type PillarVotingHistory struct {
	PillarName   string            `json:"pillar_name"`
	PillarOwner  string            `json:"pillar_owner"`
	TotalVotes   int               `json:"total_votes"`
	YesCount     int               `json:"yes_count"`
	NoCount      int               `json:"no_count"`
	AbstainCount int               `json:"abstain_count"`
	Votes        []PillarVoteEntry `json:"votes"`
}

// PillarVoteEntry is one vote cast by the pillar. ProjectName and
// PhaseName come from joins server-side so the LLM does not have to
// re-resolve IDs. Vote is the human-readable string ("yes"/"no"/
// "abstain") rather than the numeric enum so the LLM can reason
// without a lookup.
type PillarVoteEntry struct {
	ProjectID         string `json:"project_id"`
	ProjectName       string `json:"project_name,omitempty"`
	PhaseID           string `json:"phase_id,omitempty"`
	PhaseName         string `json:"phase_name,omitempty"`
	VotingID          string `json:"voting_id"`
	Vote              string `json:"vote"`
	MomentumHeight    int64  `json:"momentum_height"`
	MomentumTimestamp int64  `json:"momentum_timestamp"`
}

// VoteCodeToString translates the raw votes.vote column (0/1/2) into
// the strings used in the report DTOs. Anything outside the known
// enum becomes "unknown" rather than failing — the report should still
// surface a row that was indexed from an unexpected value.
func VoteCodeToString(code int16) string {
	switch code {
	case 0:
		return "yes"
	case 1:
		return "no"
	case 2:
		return "abstain"
	default:
		return "unknown"
	}
}

// RawProposalTally is the input shape FromProposalTally expects. It
// mirrors the internal repository row shape without importing the
// repository package (which would create a cycle: dto ← repository
// ← dto). Callers either build these by hand (tests) or copy fields
// from the matching repository type.
type RawProposalTally struct {
	VotingID      string
	ByPillar      map[string]int16
	NoVotePillars []string
}

// FromProposalTally translates a raw per-proposal tally into the wire
// DTO, expanding ByPillar into the four classification lists and
// counting each. Pillar lists are sorted for stable output (map
// iteration order is non-deterministic).
func FromProposalTally(r RawProposalTally) ProposalVoteTally {
	t := ProposalVoteTally{
		VotingID:      r.VotingID,
		NoVotePillars: append([]string(nil), r.NoVotePillars...),
		NoVoteCount:   len(r.NoVotePillars),
	}
	for name, code := range r.ByPillar {
		switch code {
		case 0:
			t.YesPillars = append(t.YesPillars, name)
		case 1:
			t.NoPillars = append(t.NoPillars, name)
		case 2:
			t.AbstainPillars = append(t.AbstainPillars, name)
		}
	}
	sort.Strings(t.YesPillars)
	sort.Strings(t.NoPillars)
	sort.Strings(t.AbstainPillars)
	sort.Strings(t.NoVotePillars)
	t.YesCount = len(t.YesPillars)
	t.NoCount = len(t.NoPillars)
	t.AbstainCount = len(t.AbstainPillars)
	return t
}

// FromPhaseTally is the per-phase wrapper around FromProposalTally.
func FromPhaseTally(phaseID, phaseName string, r RawProposalTally) PhaseVoteTally {
	return PhaseVoteTally{
		PhaseID:   phaseID,
		PhaseName: phaseName,
		Vote:      FromProposalTally(r),
	}
}

// RawPillarVote mirrors the repository's PillarVoteRow shape.
type RawPillarVote struct {
	VotingID          string
	Vote              int16
	MomentumHeight    int64
	MomentumTimestamp int64
	ProjectID         string
	PhaseID           string
	ProjectName       string
	PhaseName         string
}

// FromPillarVotingHistory assembles the public DTO from the raw rows.
// Tallies the three yes/no/abstain counters as it goes so the totals
// are guaranteed to match the rows.
func FromPillarVotingHistory(pillarName, pillarOwner string, raws []RawPillarVote) *PillarVotingHistory {
	out := &PillarVotingHistory{
		PillarName:  pillarName,
		PillarOwner: pillarOwner,
		TotalVotes:  len(raws),
		Votes:       make([]PillarVoteEntry, 0, len(raws)),
	}
	for _, r := range raws {
		switch r.Vote {
		case 0:
			out.YesCount++
		case 1:
			out.NoCount++
		case 2:
			out.AbstainCount++
		}
		out.Votes = append(out.Votes, PillarVoteEntry{
			ProjectID:         r.ProjectID,
			ProjectName:       r.ProjectName,
			PhaseID:           r.PhaseID,
			PhaseName:         r.PhaseName,
			VotingID:          r.VotingID,
			Vote:              VoteCodeToString(r.Vote),
			MomentumHeight:    r.MomentumHeight,
			MomentumTimestamp: r.MomentumTimestamp,
		})
	}
	return out
}
