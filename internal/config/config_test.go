package config

import (
	"strings"
	"testing"
	"time"
)

func TestDatabaseConfig_ConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "standard config",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "nom_indexer",
				User:     "postgres",
				Password: "secret",
			},
			expected: "postgres://postgres:secret@localhost:5432/nom_indexer?sslmode=disable",
		},
		{
			name: "custom port",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				Name:     "testdb",
				User:     "admin",
				Password: "pass123",
			},
			expected: "postgres://admin:pass123@db.example.com:5433/testdb?sslmode=disable",
		},
		{
			name: "special characters in password",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "nom_indexer",
				User:     "postgres",
				Password: "p@ss:word",
			},
			expected: "postgres://postgres:p@ss:word@localhost:5432/nom_indexer?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ConnectionString()
			if result != tt.expected {
				t.Errorf("ConnectionString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	validConfig := func() *Config {
		return &Config{
			Node: NodeConfig{
				WebSocketURL: "wss://test.example.com",
			},
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "nom_indexer",
				User:     "postgres",
				Password: "secret",
			},
		}
	}

	tests := []struct {
		name        string
		modify      func(*Config)
		expectError string
	}{
		{
			name:        "valid config",
			modify:      func(c *Config) {},
			expectError: "",
		},
		{
			name:        "missing ws_url",
			modify:      func(c *Config) { c.Node.WebSocketURL = "" },
			expectError: "node.ws_url is required",
		},
		{
			name:        "missing database host",
			modify:      func(c *Config) { c.Database.Host = "" },
			expectError: "database.host is required",
		},
		{
			name:        "invalid port - zero",
			modify:      func(c *Config) { c.Database.Port = 0 },
			expectError: "database.port must be between 1 and 65535",
		},
		{
			name:        "invalid port - negative",
			modify:      func(c *Config) { c.Database.Port = -1 },
			expectError: "database.port must be between 1 and 65535",
		},
		{
			name:        "invalid port - too high",
			modify:      func(c *Config) { c.Database.Port = 65536 },
			expectError: "database.port must be between 1 and 65535",
		},
		{
			name:        "missing database name",
			modify:      func(c *Config) { c.Database.Name = "" },
			expectError: "database.name is required",
		},
		{
			name:        "missing database user",
			modify:      func(c *Config) { c.Database.User = "" },
			expectError: "database.user is required",
		},
		{
			name:        "missing database password",
			modify:      func(c *Config) { c.Database.Password = "" },
			expectError: "database.password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(cfg)

			err := cfg.Validate()

			if tt.expectError == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.expectError)
				} else if !strings.Contains(err.Error(), tt.expectError) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.expectError)
				}
			}
		})
	}
}

func TestConfig_ValidateBoundaryPorts(t *testing.T) {
	cfg := &Config{
		Node: NodeConfig{
			WebSocketURL: "wss://test.example.com",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     1,
			Name:     "nom_indexer",
			User:     "postgres",
			Password: "secret",
		},
	}

	// Port 1 should be valid
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 1 should succeed, got: %v", err)
	}

	// Port 65535 should be valid
	cfg.Database.Port = 65535
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with port 65535 should succeed, got: %v", err)
	}
}

func TestNodesConfigFromEnvBackcompat(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "x")
	t.Setenv("API_JWT_SECRET", "y")
	t.Setenv("NODE_URL_WS", "ws://znnd:35998")
	t.Setenv("NODE_URL_FALLBACKS", "wss://my.hc1node.com:35998,https://my.hc1node.com:35997")
	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(cfg.Indexer.Nodes), 3; got != want {
		t.Fatalf("nodes len = %d, want %d", got, want)
	}
	if cfg.Indexer.Nodes[0].URL != "ws://znnd:35998" {
		t.Fatalf("primary URL: %q", cfg.Indexer.Nodes[0].URL)
	}
	if cfg.Indexer.Nodes[0].Label != "primary" {
		t.Fatalf("primary auto-label: %q", cfg.Indexer.Nodes[0].Label)
	}
	if cfg.Indexer.Nodes[2].URL != "https://my.hc1node.com:35997" {
		t.Fatalf("fallback-2 URL: %q", cfg.Indexer.Nodes[2].URL)
	}
}

func TestNodesFallbacksSkipEmptySegments(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "x")
	t.Setenv("API_JWT_SECRET", "y")
	t.Setenv("NODE_URL_WS", "ws://znnd:35998")
	t.Setenv("NODE_URL_FALLBACKS", "wss://a.example.com:35998,,https://b.example.com:35997")
	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(cfg.Indexer.Nodes), 3; got != want {
		t.Fatalf("nodes len = %d, want %d", got, want)
	}
	if cfg.Indexer.Nodes[1].Label != "fallback-1" {
		t.Fatalf("nodes[1].Label = %q, want fallback-1", cfg.Indexer.Nodes[1].Label)
	}
	if cfg.Indexer.Nodes[2].Label != "fallback-2" {
		t.Fatalf("nodes[2].Label = %q, want fallback-2 (sequential, not fallback-3)",
			cfg.Indexer.Nodes[2].Label)
	}
}

func TestNodesFallbacksDeriveProbeURL(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "x")
	t.Setenv("API_JWT_SECRET", "y")
	t.Setenv("NODE_URL_WS", "ws://znnd:35998")
	t.Setenv("NODE_URL_FALLBACKS", "wss://my.hc1node.com:35998")
	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Indexer.Nodes[0].ProbeURL; got != "http://znnd:35997" {
		t.Fatalf("primary ProbeURL = %q", got)
	}
	if got := cfg.Indexer.Nodes[1].ProbeURL; got != "https://my.hc1node.com:35997" {
		t.Fatalf("fallback ProbeURL = %q", got)
	}
}

func TestNodesFallbacksDoNotDeriveWhenPortMissing(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "x")
	t.Setenv("API_JWT_SECRET", "y")
	t.Setenv("NODE_URL_WS", "wss://test.hc1node.com")
	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Indexer.Nodes[0].ProbeURL; got != "" {
		t.Fatalf("expected no ProbeURL derivation when port absent, got %q", got)
	}
}

func TestWatchdogConfigDefaults(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "x")
	t.Setenv("API_JWT_SECRET", "y")
	t.Setenv("NODE_URL_WS", "ws://znnd:35998")
	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Indexer.Watchdog.Interval != 30*time.Second {
		t.Fatalf("default interval: %v", cfg.Indexer.Watchdog.Interval)
	}
	if cfg.Indexer.Watchdog.UnhealthyStreak != 2 {
		t.Fatalf("default unhealthy_streak: %d", cfg.Indexer.Watchdog.UnhealthyStreak)
	}
	if cfg.Indexer.Watchdog.FailbackStreak != 5 {
		t.Fatalf("default failback_streak: %d", cfg.Indexer.Watchdog.FailbackStreak)
	}
	if cfg.Indexer.Health.Port != 9092 {
		t.Fatalf("default health port: %d", cfg.Indexer.Health.Port)
	}
}
