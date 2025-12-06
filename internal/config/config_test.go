package config

import (
	"strings"
	"testing"
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
