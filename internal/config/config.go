package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
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

// IndexerConfig groups the indexer-process-only settings: the prioritized
// list of upstream nodes, the sync watchdog policy, and the indexer's
// own HTTP health server. The API and MCP processes do not consult it.
type IndexerConfig struct {
	Nodes    []NodeEntry    `mapstructure:"nodes"`
	Watchdog WatchdogConfig `mapstructure:"watchdog"`
	Health   HealthConfig   `mapstructure:"health"`
}

// NodeEntry is one upstream Zenon node. URL accepts ws://, wss://,
// http://, or https://; the subscription requires WS, but probes can
// use either. Label appears in logs and the indexer_sync_status row.
//
// ProbeURL is optional and overrides the JSON-RPC endpoint used by the
// watchdog's HTTP probes. When unset, the probe endpoint defaults to
// rewriting URL's scheme (ws→http, wss→https) with the port preserved.
// Set ProbeURL explicitly when the node splits its WS and HTTP-RPC
// listeners across different ports — the canonical Zenon convention is
// WS on N, HTTP-RPC on N-1 (e.g. WS=35998, HTTP=35997).
type NodeEntry struct {
	URL      string `mapstructure:"url"`
	Label    string `mapstructure:"label"`
	ProbeURL string `mapstructure:"probe_url"` // optional; HTTP JSON-RPC endpoint for watchdog probes
}

// WatchdogConfig controls the sync watchdog goroutine. See
// docs/operations/watchdog.md for failure-mode coverage and the
// rationale behind the asymmetric streak thresholds.
type WatchdogConfig struct {
	// Enabled turns the watchdog on. When false, drift detection and
	// node failover are disabled — existing SDK reconnect behavior
	// is unaffected.
	Enabled bool `mapstructure:"enabled"`
	// Interval is the cadence of the watchdog tick.
	Interval time.Duration `mapstructure:"interval"`
	// StallThreshold flags a stall when no momentum has been committed
	// for this long. Measured against an atomic Unix-seconds counter
	// updated by the subscription loop.
	StallThreshold time.Duration `mapstructure:"stall_threshold"`
	// IndexerDriftThreshold is the maximum acceptable gap (in momentums)
	// between znnd's frontier and the DB's last indexed height before
	// the watchdog flags indexer_lagging and forces a subscription restart.
	IndexerDriftThreshold int64 `mapstructure:"indexer_drift_threshold"`
	// NodeDriftThreshold is the maximum acceptable gap (in momentums)
	// between znnd's targetHeight (what its peers know) and currentHeight
	// (what it has) before the watchdog flags node_lagging.
	NodeDriftThreshold int64 `mapstructure:"node_drift_threshold"`
	// UnhealthyStreak is how many consecutive bad ticks trigger a
	// failover to the next configured node. Lower = faster failover.
	UnhealthyStreak int `mapstructure:"unhealthy_streak"`
	// FailbackStreak is how many consecutive healthy probes of a
	// higher-priority node are required before failing back to it.
	// Should be > UnhealthyStreak to prevent flapping.
	FailbackStreak int `mapstructure:"failback_streak"`
}

// HealthConfig configures the indexer-side HTTP server that exposes
// /healthz (process alive) and /readyz (caught up). Internal-only;
// the docker compose healthcheck probes it.
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
	_ = v.BindEnv("indexer.watchdog.stall_threshold", "INDEXER_WATCHDOG_STALL_THRESHOLD")
	_ = v.BindEnv("indexer.watchdog.indexer_drift_threshold", "INDEXER_WATCHDOG_INDEXER_DRIFT_THRESHOLD")
	_ = v.BindEnv("indexer.watchdog.node_drift_threshold", "INDEXER_WATCHDOG_NODE_DRIFT_THRESHOLD")
	_ = v.BindEnv("indexer.watchdog.unhealthy_streak", "INDEXER_WATCHDOG_UNHEALTHY_STREAK")
	_ = v.BindEnv("indexer.watchdog.failback_streak", "INDEXER_WATCHDOG_FAILBACK_STREAK")
	_ = v.BindEnv("indexer.health.enabled", "INDEXER_HEALTH_ENABLED")
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
	if err := v.Unmarshal(&cfg, viper.DecodeHook(
		mapstructure.StringToTimeDurationHookFunc(),
	)); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// If indexer.nodes wasn't set by YAML, build it from the legacy
	// NODE_URL_WS (primary) + NODE_URL_FALLBACKS (comma-separated).
	if len(cfg.Indexer.Nodes) == 0 && cfg.Node.WebSocketURL != "" {
		primary := NodeEntry{
			URL:   cfg.Node.WebSocketURL,
			Label: "primary",
		}
		if derived := deriveProbeURL(cfg.Node.WebSocketURL); derived != "" {
			primary.ProbeURL = derived
		}
		cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, primary)
		if fb := os.Getenv("NODE_URL_FALLBACKS"); fb != "" {
			seq := 1
			for _, u := range strings.Split(fb, ",") {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				entry := NodeEntry{
					URL:   u,
					Label: fmt.Sprintf("fallback-%d", seq),
				}
				if derived := deriveProbeURL(u); derived != "" {
					entry.ProbeURL = derived
				}
				cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, entry)
				seq++
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

// deriveProbeURL applies the canonical Zenon port convention (WS=N,
// HTTP-RPC=N-1) to fill in the JSON-RPC probe URL when the operator
// didn't set one explicitly. Only fires on the well-known WS port
// 35998 so the heuristic stays predictable; other ports return "" and
// callers fall back to the scheme-only rewrite that preserves the port.
func deriveProbeURL(wsURL string) string {
	u, err := url.Parse(wsURL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return ""
	}
	port := u.Port()
	if port == "" {
		return "" // no explicit port → don't try
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum != 35998 {
		return ""
	}
	scheme := "http"
	if u.Scheme == "wss" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d%s", scheme, u.Hostname(), portNum-1, u.Path)
}
