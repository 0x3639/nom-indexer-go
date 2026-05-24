// Package oapi holds the generated OpenAPI types from docs/api/openapi.yaml.
//
// The source of truth for the API contract is docs/api/openapi.yaml.
// `go generate ./internal/api/oapi/...` (or `go generate ./...`)
// regenerates server.gen.go. The generated file is committed so builds
// work without first running the generator.
//
// Drift enforcement: the v1 API does not wire the generated
// ServerInterface into handlers — chi-style http.HandlerFunc keeps
// dependency injection simpler and avoids the extra layer of generated
// glue. Instead, internal/api/router/router_test.go parses openapi.yaml
// at test time and asserts every documented GET path is registered on
// the router (and vice versa). That test is the spec-vs-implementation
// gate; a new endpoint that's documented but not registered (or
// registered but not documented) fails CI.
//
// The version is pinned in the directive so `go generate` works
// regardless of whether the generator module is present in go.mod
// (only the runtime is — see go.mod).
package oapi

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0 --config=oapi-codegen.yaml ../../../docs/api/openapi.yaml
