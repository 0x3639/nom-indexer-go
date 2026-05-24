package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

type Project struct {
	ID                  string `json:"id"`
	VotingID            string `json:"voting_id"`
	Owner               string `json:"owner"`
	Name                string `json:"name"`
	Description         string `json:"description,omitempty"`
	URL                 string `json:"url,omitempty"`
	ZnnFundsNeeded      Amount `json:"znn_funds_needed"`
	QsrFundsNeeded      Amount `json:"qsr_funds_needed"`
	CreationTimestamp   int64  `json:"creation_timestamp"`
	LastUpdateTimestamp int64  `json:"last_update_timestamp"`
	Status              int16  `json:"status"`
	YesVotes            int16  `json:"yes_votes"`
	NoVotes             int16  `json:"no_votes"`
	TotalVotes          int16  `json:"total_votes"`
}

func FromProject(p *models.Project) *Project {
	if p == nil {
		return nil
	}
	return &Project{
		ID:                  p.ID,
		VotingID:            p.VotingID,
		Owner:               p.Owner,
		Name:                p.Name,
		Description:         p.Description,
		URL:                 p.URL,
		ZnnFundsNeeded:      AmountFromInt64(p.ZnnFundsNeeded),
		QsrFundsNeeded:      AmountFromInt64(p.QsrFundsNeeded),
		CreationTimestamp:   p.CreationTimestamp,
		LastUpdateTimestamp: p.LastUpdateTimestamp,
		Status:              p.Status,
		YesVotes:            p.YesVotes,
		NoVotes:             p.NoVotes,
		TotalVotes:          p.TotalVotes,
	}
}

func FromProjects(in []*models.Project) []*Project {
	out := make([]*Project, 0, len(in))
	for _, p := range in {
		if d := FromProject(p); d != nil {
			out = append(out, d)
		}
	}
	return out
}

type ProjectPhase struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	VotingID          string `json:"voting_id"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	URL               string `json:"url,omitempty"`
	ZnnFundsNeeded    Amount `json:"znn_funds_needed"`
	QsrFundsNeeded    Amount `json:"qsr_funds_needed"`
	CreationTimestamp int64  `json:"creation_timestamp"`
	AcceptedTimestamp int64  `json:"accepted_timestamp"`
	Status            int16  `json:"status"`
	YesVotes          int16  `json:"yes_votes"`
	NoVotes           int16  `json:"no_votes"`
	TotalVotes        int16  `json:"total_votes"`
}

func FromProjectPhase(p *models.ProjectPhase) *ProjectPhase {
	if p == nil {
		return nil
	}
	return &ProjectPhase{
		ID:                p.ID,
		ProjectID:         p.ProjectID,
		VotingID:          p.VotingID,
		Name:              p.Name,
		Description:       p.Description,
		URL:               p.URL,
		ZnnFundsNeeded:    AmountFromInt64(p.ZnnFundsNeeded),
		QsrFundsNeeded:    AmountFromInt64(p.QsrFundsNeeded),
		CreationTimestamp: p.CreationTimestamp,
		AcceptedTimestamp: p.AcceptedTimestamp,
		Status:            p.Status,
		YesVotes:          p.YesVotes,
		NoVotes:           p.NoVotes,
		TotalVotes:        p.TotalVotes,
	}
}

func FromProjectPhases(in []*models.ProjectPhase) []*ProjectPhase {
	out := make([]*ProjectPhase, 0, len(in))
	for _, p := range in {
		if d := FromProjectPhase(p); d != nil {
			out = append(out, d)
		}
	}
	return out
}
