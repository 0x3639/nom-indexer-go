package config

import (
	"reflect"
	"testing"
)

func TestAPIConfig_CORSAllowedOriginsList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty returns nil", "", nil},
		{"whitespace-only returns nil", "  ,  ,", nil},
		{"single origin", "https://example.com", []string{"https://example.com"}},
		{"comma-separated", "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
		{"trims spaces", " https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
		{"drops empty entries", "https://a.com,,https://b.com,", []string{"https://a.com", "https://b.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &APIConfig{CORSAllowedOrigins: tt.in}
			if got := c.CORSAllowedOriginsList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CORSAllowedOriginsList() = %v, want %v", got, tt.want)
			}
		})
	}
}
