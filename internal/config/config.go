package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Node              NodeConfig     `mapstructure:"node"`
	Database          DatabaseConfig `mapstructure:"database"`
	Logging           LoggingConfig  `mapstructure:"logging"`
	Cron              CronConfig     `mapstructure:"cron"`
	API               APIConfig      `mapstructure:"api"`
	MCP               MCPConfig      `mapstructure:"mcp"`
	Indexer           IndexerConfig  `mapstructure:"indexer"`
	BackfillOnStartup bool           `mapstructure:"backfill_on_startup"`
}

type IndexerConfig struct {
	Nodes    []NodeEntry    `mapstructure:"nodes"`
	Watchdog WatchdogConfig `mapstructure:"watchdog"`
	Health   HealthConfig   `mapstructure:"health"`
}

type NodeEntry struct {
	URL   string `mapstructure:"url"`
	Label string `mapstructure:"label"`
}

type WatchdogConfig struct {
	Enabled                 bool          `mapstructure:"enabled"`
	Interval                time.Duration `mapstructure:"interval"`
	StallThreshold          time.Duration `mapstructure:"stall_threshold"`
	IndexerDriftThreshold   int64         `mapstructure:"indexer_drift_threshold"`
	NodeDriftThreshold      int64         `mapstructure:"node_drift_threshold"`
	UnhealthyStreak         int           `mapstructure:"unhealthy_streak"`
	FailbackStreak          int           `mapstructure:"failback_streak"`
	TolerateMissingSyncInfo bool          `mapstructure:"tolerate_missing_syncinfo"`
}

type HealthConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

type NodeConfig struct {
	WebSocketURL string `mapstructure:"ws_url"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	PoolSize int    `mapstructure:"pool_size"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type CronConfig struct {
	VotingActivityInterval string `mapstructure:"voting_activity_interval"`
	TokenHoldersInterval   string `mapstructure:"token_holders_interval"`
}

// APIConfig controls the HTTP API server (cmd/api).
type APIConfig struct {
	// Port is the public listener port for the API.
	Port int `mapstructure:"port"`
	// MetricsPort is the separate listener for /metrics; bound to 0.0.0.0
	// but typically scoped to a private network in deployment.
	MetricsPort int `mapstructure:"metrics_port"`
	// JWTSecret is the HS256 signing secret. Required when the API runs.
	JWTSecret string `mapstructure:"jwt_secret"`
	// CORSAllowedOrigins is a comma-separated allowlist; empty = deny.
	CORSAllowedOrigins string `mapstructure:"cors_allowed_origins"`
	// RateLimitPerMinute caps requests per JWT subject (or IP if unauthenticated).
	RateLimitPerMinute int `mapstructure:"rate_limit_per_minute"`
}

// CORSAllowedOriginsList parses CORSAllowedOrigins into a trimmed slice.
// Returns nil if the field is empty or contains no non-empty entries.
func (a *APIConfig) CORSAllowedOriginsList() []string {
	if a.CORSAllowedOrigins == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(a.CORSAllowedOrigins, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// MCPConfig controls the MCP server (cmd/mcp). Independent ports from
// the REST API so the two services can run side-by-side in the same
// compose stack with no coupling.
type MCPConfig struct {
	// Port is the public listener for the Streamable HTTP transport at
	// POST /mcp. Defaults to 8081.
	Port int `mapstructure:"port"`
	// MetricsPort is the separate listener for /metrics; bound to
	// 0.0.0.0 but typically scoped to a private network in deployment.
	// Defaults to 9091 (the REST API uses 9090).
	MetricsPort int `mapstructure:"metrics_port"`
	// JWTSecret is the HS256 signing secret. If empty, the MCP server
	// falls back to APIConfig.JWTSecret — convenient default so
	// admins manage one secret + one cmd/jwt-issue invocation. Set
	// explicitly only when operators want isolated key material per
	// service.
	JWTSecret string `mapstructure:"jwt_secret"`
	// CORSAllowedOrigins is a comma-separated allowlist; empty = deny.
	// Most consumers (Claude Desktop, Claude Code) are HTTP clients
	// without an Origin header, so CORS only matters for browser-based
	// MCP clients.
	CORSAllowedOrigins string `mapstructure:"cors_allowed_origins"`
	// RateLimitPerMinute caps MCP transport requests per JWT subject. Defaults
	// to 60 (matches the REST API).
	RateLimitPerMinute int `mapstructure:"rate_limit_per_minute"`
}

// CORSAllowedOriginsList parses CORSAllowedOrigins into a trimmed slice.
// Returns nil if the field is empty or contains no non-empty entries.
func (m *MCPConfig) CORSAllowedOriginsList() []string {
	if m.CORSAllowedOrigins == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(m.CORSAllowedOrigins, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// EffectiveJWTSecret returns the MCP-specific secret if set, else the
// API's secret. Operators get a single-secret default; isolation is
// opt-in by setting MCP_JWT_SECRET explicitly.
func (m *MCPConfig) EffectiveJWTSecret(apiSecret string) string {
	if m.JWTSecret != "" {
		return m.JWTSecret
	}
	return apiSecret
}

// ConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, d.Password, d.Host, d.Port, d.Name)
}

// Load loads configuration from environment variables and config file
func Load() (*Config, error) {
	v := viper.New()

	// Set config file details
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/app")

	// Set defaults
	v.SetDefault("node.ws_url", "wss://test.hc1node.com")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "nom_indexer")
	v.SetDefault("database.user", "postgres")
	// Note: No default for database.password - set via config.yaml or DATABASE_PASSWORD env var
	v.SetDefault("database.pool_size", 10)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")
	v.SetDefault("cron.voting_activity_interval", "10m")
	v.SetDefault("cron.token_holders_interval", "10m")
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.metrics_port", 9090)
	v.SetDefault("api.cors_allowed_origins", "")
	v.SetDefault("api.rate_limit_per_minute", 60)
	// Note: No default for api.jwt_secret - set via config.yaml or API_JWT_SECRET env var.
	v.SetDefault("mcp.port", 8081)
	v.SetDefault("mcp.metrics_port", 9091)
	v.SetDefault("mcp.cors_allowed_origins", "")
	v.SetDefault("mcp.rate_limit_per_minute", 60)
	// mcp.jwt_secret defaults to empty → falls back to api.jwt_secret
	// (see MCPConfig.EffectiveJWTSecret). Set MCP_JWT_SECRET only for
	// independent key material.
	v.SetDefault("backfill_on_startup", false)
	v.SetDefault("indexer.watchdog.enabled", false)
	v.SetDefault("indexer.watchdog.interval", "30s")
	v.SetDefault("indexer.watchdog.stall_threshold", "60s")
	v.SetDefault("indexer.watchdog.indexer_drift_threshold", 3)
	v.SetDefault("indexer.watchdog.node_drift_threshold", 3)
	v.SetDefault("indexer.watchdog.unhealthy_streak", 2)
	v.SetDefault("indexer.watchdog.failback_streak", 5)
	v.SetDefault("indexer.watchdog.tolerate_missing_syncinfo", true)
	v.SetDefault("indexer.health.enabled", true)
	v.SetDefault("indexer.health.port", 9092)

	// Enable environment variable binding
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind specific environment variables (these take precedence)
	// Note: BindEnv errors are ignored as they only fail on invalid key names
	_ = v.BindEnv("node.ws_url", "NODE_URL_WS")
	_ = v.BindEnv("database.host", "DATABASE_ADDRESS")
	_ = v.BindEnv("database.port", "DATABASE_PORT")
	_ = v.BindEnv("database.name", "DATABASE_NAME")
	_ = v.BindEnv("database.user", "DATABASE_USERNAME")
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("logging.level", "LOG_LEVEL")
	_ = v.BindEnv("logging.format", "LOG_FORMAT")
	_ = v.BindEnv("backfill_on_startup", "BACKFILL_ON_STARTUP")
	_ = v.BindEnv("api.port", "API_PORT")
	_ = v.BindEnv("api.metrics_port", "API_METRICS_PORT")
	_ = v.BindEnv("api.jwt_secret", "API_JWT_SECRET")
	_ = v.BindEnv("api.cors_allowed_origins", "API_CORS_ALLOWED_ORIGINS")
	_ = v.BindEnv("api.rate_limit_per_minute", "API_RATE_LIMIT_PER_MINUTE")
	_ = v.BindEnv("mcp.port", "MCP_PORT")
	_ = v.BindEnv("mcp.metrics_port", "MCP_METRICS_PORT")
	_ = v.BindEnv("mcp.jwt_secret", "MCP_JWT_SECRET")
	_ = v.BindEnv("mcp.cors_allowed_origins", "MCP_CORS_ALLOWED_ORIGINS")
	_ = v.BindEnv("mcp.rate_limit_per_minute", "MCP_RATE_LIMIT_PER_MINUTE")
	_ = v.BindEnv("indexer.watchdog.enabled", "INDEXER_WATCHDOG_ENABLED")
	_ = v.BindEnv("indexer.watchdog.interval", "INDEXER_WATCHDOG_INTERVAL")
	_ = v.BindEnv("indexer.health.port", "INDEXER_HEALTH_PORT")

	// Try to read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use env vars and defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// If indexer.nodes wasn't set by YAML, build it from the legacy
	// NODE_URL_WS (primary) + NODE_URL_FALLBACKS (comma-separated).
	if len(cfg.Indexer.Nodes) == 0 && cfg.Node.WebSocketURL != "" {
		cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, NodeEntry{
			URL:   cfg.Node.WebSocketURL,
			Label: "primary",
		})
		if fb := os.Getenv("NODE_URL_FALLBACKS"); fb != "" {
			for i, u := range strings.Split(fb, ",") {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, NodeEntry{
					URL:   u,
					Label: fmt.Sprintf("fallback-%d", i+1),
				})
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// BuildLogger constructs a zap logger from the LoggingConfig. Unknown levels
// fall back to info; unknown formats fall back to console.
func (l *LoggingConfig) BuildLogger() (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if l.Level != "" {
		if err := level.UnmarshalText([]byte(strings.ToLower(l.Level))); err != nil {
			return nil, fmt.Errorf("invalid logging.level %q: %w", l.Level, err)
		}
	}

	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(level)
	switch strings.ToLower(l.Format) {
	case "json":
		zc.Encoding = "json"
	case "", "console":
		zc.Encoding = "console"
		zc.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zc.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default:
		return nil, fmt.Errorf("invalid logging.format %q (want json or console)", l.Format)
	}

	return zc.Build()
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Node.WebSocketURL == "" {
		return fmt.Errorf("node.ws_url is required")
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}

	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}

	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}

	if c.Database.Password == "" {
		return fmt.Errorf("database.password is required (set in config.yaml or DATABASE_PASSWORD env var)")
	}

	return nil
}
