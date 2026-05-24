package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

type Vote struct {
	ID                int    `json:"id"`
	MomentumHash      string `json:"momentum_hash"`
	MomentumTimestamp int64  `json:"momentum_timestamp"`
	MomentumHeight    int64  `json:"momentum_height"`
	VoterAddress      string `json:"voter_address"`
	ProjectID         string `json:"project_id"`
	PhaseID           string `json:"phase_id,omitempty"`
	VotingID          string `json:"voting_id"`
	Vote              int16  `json:"vote"`
}

func FromVote(v *models.Vote) *Vote {
	if v == nil {
		return nil
	}
	return &Vote{
		ID:                v.ID,
		MomentumHash:      v.MomentumHash,
		MomentumTimestamp: v.MomentumTimestamp,
		MomentumHeight:    v.MomentumHeight,
		VoterAddress:      v.VoterAddress,
		ProjectID:         v.ProjectID,
		PhaseID:           v.PhaseID,
		VotingID:          v.VotingID,
		Vote:              v.Vote,
	}
}

func FromVotes(in []*models.Vote) []*Vote {
	out := make([]*Vote, 0, len(in))
	for _, v := range in {
		if d := FromVote(v); d != nil {
			out = append(out, d)
		}
	}
	return out
}
