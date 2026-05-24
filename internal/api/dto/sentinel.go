package dto

import "github.com/0x3639/nom-indexer-go/internal/models"

type Sentinel struct {
	Owner                 string `json:"owner"`
	RegistrationTimestamp int64  `json:"registration_timestamp"`
	IsRevocable           bool   `json:"is_revocable"`
	RevokeCooldown        string `json:"revoke_cooldown,omitempty"`
	Active                bool   `json:"active"`
}

func FromSentinel(s *models.Sentinel) *Sentinel {
	if s == nil {
		return nil
	}
	return &Sentinel{
		Owner:                 s.Owner,
		RegistrationTimestamp: s.RegistrationTimestamp,
		IsRevocable:           s.IsRevocable,
		RevokeCooldown:        s.RevokeCooldown,
		Active:                s.Active,
	}
}

func FromSentinels(in []*models.Sentinel) []*Sentinel {
	out := make([]*Sentinel, 0, len(in))
	for _, s := range in {
		if d := FromSentinel(s); d != nil {
			out = append(out, d)
		}
	}
	return out
}
