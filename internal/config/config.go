package config

import (
	"errors"
	"fmt"
	"strings"

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
	BackfillOnStartup bool           `mapstructure:"backfill_on_startup"`
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
	v.SetDefault("backfill_on_startup", false)

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

	// Try to read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use env vars and defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
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
