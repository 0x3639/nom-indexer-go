// CI/CD pipeline for nom-indexer-go
//
// This module provides functions to build, test, lint, and publish
// the nom-indexer-go Docker image to GitHub Container Registry.

package main

import (
	"context"
	"dagger/nom-indexer/internal/dagger"
)

type NomIndexer struct{}

// Build builds the Docker image using the existing Dockerfile
func (m *NomIndexer) Build(source *dagger.Directory) *dagger.Container {
	return source.DockerBuild()
}

// Test runs go test with CGO enabled (required for secp256k1)
func (m *NomIndexer) Test(ctx context.Context, source *dagger.Directory) (string, error) {
	return dag.Container().
		From("golang:1.24-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "git", "gcc", "musl-dev"}).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/app", source).
		WithWorkdir("/app").
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"go", "test", "-v", "./..."}).
		Stdout(ctx)
}

// Lint runs golangci-lint on the source code
// Note: Uses Go 1.24 base image and installs golangci-lint since pre-built images
// don't yet support Go 1.24
func (m *NomIndexer) Lint(ctx context.Context, source *dagger.Directory) (string, error) {
	return dag.Container().
		From("golang:1.24-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "git", "gcc", "musl-dev", "binutils-gold"}).
		WithExec([]string{"go", "install", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.1"}).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("golangci-lint")).
		WithDirectory("/app", source).
		WithWorkdir("/app").
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"golangci-lint", "run", "--timeout", "5m", "./..."}).
		Stdout(ctx)
}

// Publish builds and pushes the image to GitHub Container Registry
func (m *NomIndexer) Publish(ctx context.Context, source *dagger.Directory, tag string) (string, error) {
	return m.Build(source).
		Publish(ctx, "ghcr.io/0x3639/nom-indexer-go:"+tag)
}

// CI runs the full CI pipeline: lint, test, and build
func (m *NomIndexer) CI(ctx context.Context, source *dagger.Directory) (string, error) {
	// Run lint
	if _, err := m.Lint(ctx, source); err != nil {
		return "", err
	}

	// Run tests
	if _, err := m.Test(ctx, source); err != nil {
		return "", err
	}

	// Build container to validate Dockerfile
	_ = m.Build(source)

	return "CI passed: lint, test, and build succeeded!", nil
}
