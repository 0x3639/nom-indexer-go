package indexer

import (
	"testing"
	"time"
)

func TestParseCronInterval(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		def     time.Duration
		want    time.Duration
		wantErr bool
	}{
		{name: "empty returns default", input: "", def: 10 * time.Minute, want: 10 * time.Minute},
		{name: "minutes", input: "5m", def: time.Minute, want: 5 * time.Minute},
		{name: "hours", input: "2h", def: time.Minute, want: 2 * time.Hour},
		{name: "seconds", input: "30s", def: time.Minute, want: 30 * time.Second},
		{name: "compound", input: "1h30m", def: 0, want: 90 * time.Minute},
		{name: "bad format errors", input: "ten minutes", def: time.Minute, wantErr: true},
		{name: "trailing garbage errors", input: "10m garbage", def: time.Minute, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCronInterval(tt.input, tt.def)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseCronInterval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
