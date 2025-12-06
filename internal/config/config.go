package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Node     NodeConfig     `mapstructure:"node"`
	Database DatabaseConfig `mapstructure:"database"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Cron     CronConfig     `mapstructure:"cron"`
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

	// Try to read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
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
