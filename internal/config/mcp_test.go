package config

import (
	"reflect"
	"testing"
)

func TestMCPConfig_CORSAllowedOriginsList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty returns nil", "", nil},
		{"whitespace-only returns nil", "  ,  ,", nil},
		{"single origin", "https://desktop.anthropic.com", []string{"https://desktop.anthropic.com"}},
		{"comma-separated", "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
		{"trims spaces", " https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
		{"drops empty entries", "https://a.com,,https://b.com,", []string{"https://a.com", "https://b.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &MCPConfig{CORSAllowedOrigins: tt.in}
			if got := c.CORSAllowedOriginsList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CORSAllowedOriginsList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPConfig_EffectiveJWTSecret(t *testing.T) {
	tests := []struct {
		name      string
		mcpSecret string
		apiSecret string
		want      string
	}{
		{
			name:      "mcp set wins over api",
			mcpSecret: "mcp-only-key",
			apiSecret: "api-key",
			want:      "mcp-only-key",
		},
		{
			name:      "mcp empty falls back to api",
			mcpSecret: "",
			apiSecret: "shared-api-key",
			want:      "shared-api-key",
		},
		{
			name:      "both empty returns empty (caller refuses to boot)",
			mcpSecret: "",
			apiSecret: "",
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MCPConfig{JWTSecret: tt.mcpSecret}
			if got := m.EffectiveJWTSecret(tt.apiSecret); got != tt.want {
				t.Errorf("EffectiveJWTSecret() = %q, want %q", got, tt.want)
			}
		})
	}
}
