// Package models declares the Go struct mirrors of every table in the
// schema, along with enum types (RewardType) and address constants for
// embedded contracts and well-known sentinels.
//
// This package imports only the standard library — deliberately. It is
// the leaf of the import graph: internal/api/dto already depends on it
// (and the future MCP package will too) without pulling in pgx, viper,
// zap, or the SDK.
//
// See docs/schema/index.md for the canonical contract these structs
// mirror and docs/reference/addresses.md for the constants.
package models
