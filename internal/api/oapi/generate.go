// Package oapi holds the generated OpenAPI server interface + types.
//
// The source of truth for the API contract is docs/api/openapi.yaml.
// `go generate ./internal/api/oapi/...` (or `go generate ./...`) regenerates
// server.gen.go from that spec. The generated file is committed so builds
// work without first running the generator.
//
// Hand-written handlers in internal/api/handlers/ implement the
// ServerInterface produced here — the compiler enforces that every spec'd
// operation has an implementation.
package oapi

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=oapi-codegen.yaml ../../../docs/api/openapi.yaml
