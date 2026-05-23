// Package config loads runtime configuration for nom-indexer-go.
//
// Sources are layered (later wins): hard-coded defaults, optional
// config.yaml at the project root or /app, then environment variables.
// Viper handles the discovery; this package validates the result and
// constructs the zap logger from the LoggingConfig.
//
// See docs/config/reference.md for every field and env var.
package config
