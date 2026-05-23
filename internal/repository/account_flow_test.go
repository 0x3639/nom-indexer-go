package repository

import (
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestFlowColumn(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		direction string
		want      string
	}{
		{"ZNN sent", models.ZnnTokenStandard, "sent", "znn_sent"},
		{"ZNN received", models.ZnnTokenStandard, "received", "znn_received"},
		{"QSR sent", models.QsrTokenStandard, "sent", "qsr_sent"},
		{"QSR received", models.QsrTokenStandard, "received", "qsr_received"},
		{"random token sent", "zts1foobarxxxxxxxxxxxxxxxxxx", "sent", ""},
		{"empty token standard", "", "received", ""},
		{"empty direction defaults to received", models.ZnnTokenStandard, "", "znn_received"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flowColumn(tt.token, tt.direction); got != tt.want {
				t.Errorf("flowColumn(%q, %q) = %q, want %q", tt.token, tt.direction, got, tt.want)
			}
		})
	}
}
