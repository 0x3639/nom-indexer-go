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

// Build builds the indexer Docker image using the default Dockerfile.
func (m *NomIndexer) Build(source *dagger.Directory) *dagger.Container {
	return source.DockerBuild()
}

// BuildAPI builds the API Docker image (Dockerfile.api, CGO_ENABLED=0).
// Keeps the API image small and proves the API never accidentally
// imports anything that needs CGO.
func (m *NomIndexer) BuildAPI(source *dagger.Directory) *dagger.Container {
	return source.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile.api",
	})
}

// PublishAPI builds and pushes the API image to GitHub Container Registry.
func (m *NomIndexer) PublishAPI(ctx context.Context, source *dagger.Directory, tag string) (string, error) {
	return m.BuildAPI(source).
		Publish(ctx, "ghcr.io/0x3639/nom-indexer-api:"+tag)
}

// BuildMCP builds the MCP server Docker image (Dockerfile.mcp,
// CGO_ENABLED=0). Same shape as BuildAPI — the MCP binary never imports
// go-zenon, so a CGO-free build keeps the runtime image small.
func (m *NomIndexer) BuildMCP(source *dagger.Directory) *dagger.Container {
	return source.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Dockerfile: "Dockerfile.mcp",
	})
}

// PublishMCP builds and pushes the MCP image to GitHub Container Registry.
func (m *NomIndexer) PublishMCP(ctx context.Context, source *dagger.Directory, tag string) (string, error) {
	return m.BuildMCP(source).
		Publish(ctx, "ghcr.io/0x3639/nom-indexer-mcp:"+tag)
}

// Test runs go test with CGO enabled (required for secp256k1)
func (m *NomIndexer) Test(ctx context.Context, source *dagger.Directory) (string, error) {
	return dag.Container().
		From("golang:1.25-alpine").
		WithExec([]string{"apk", "add", "--no-cache", "git", "gcc", "musl-dev"}).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("go-build")).
		WithDirectory("/app", source).
		WithWorkdir("/app").
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"go", "test", "-v", "./..."}).
		Stdout(ctx)
}

// Lint runs golangci-lint on the source code.
// golangci-lint is built from source inside the golang:1.25 base image
// so it tracks the Go toolchain version pinned in go.mod (1.25). The
// official pre-built binaries lag the latest Go release by several weeks.
func (m *NomIndexer) Lint(ctx context.Context, source *dagger.Directory) (string, error) {
	return dag.Container().
		From("golang:1.25-alpine").
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

// DocsSync verifies the committed llms.txt and llms-full.txt still match
// the docs/ tree. This mirrors the "Verify llms.txt is in sync" step in
// .github/workflows/docs-pr-preview.yml so that `dagger call ci` catches
// docs drift the Go pipeline (lint/test/build) cannot see. PyYAML (pulled
// in via scripts/docs/requirements.txt) is the only runtime dependency of
// the generators; the full requirements pin keeps the environment in lockstep
// with the GitHub job.
func (m *NomIndexer) DocsSync(ctx context.Context, source *dagger.Directory) (string, error) {
	return dag.Container().
		From("python:3.12-slim").
		WithMountedCache("/root/.cache/pip", dag.CacheVolume("pip")).
		WithDirectory("/app", source).
		WithWorkdir("/app").
		WithExec([]string{"pip", "install", "-r", "scripts/docs/requirements.txt"}).
		WithExec([]string{"python", "scripts/docs/gen-llms-txt.py", "--check"}).
		WithExec([]string{"python", "scripts/docs/gen-llms-full.py", "--check"}).
		Stdout(ctx)
}

// Publish builds and pushes the image to GitHub Container Registry
func (m *NomIndexer) Publish(ctx context.Context, source *dagger.Directory, tag string) (string, error) {
	return m.Build(source).
		Publish(ctx, "ghcr.io/0x3639/nom-indexer-go:"+tag)
}

// CI runs the full CI pipeline: lint, test, docs-sync, and build
func (m *NomIndexer) CI(ctx context.Context, source *dagger.Directory) (string, error) {
	// Run lint
	if _, err := m.Lint(ctx, source); err != nil {
		return "", err
	}

	// Run tests
	if _, err := m.Test(ctx, source); err != nil {
		return "", err
	}

	// Verify llms.txt / llms-full.txt are in sync with docs/ — matches the
	// docs-pr-preview GitHub workflow so docs drift fails here too.
	if _, err := m.DocsSync(ctx, source); err != nil {
		return "", err
	}

	// Build both container images. Dagger graph nodes are lazy —
	// assigning to _ never materializes the build, so Dockerfile
	// regressions used to pass CI silently. Sync(ctx) forces the
	// container to actually build.
	if _, err := m.Build(source).Sync(ctx); err != nil {
		return "", err
	}
	if _, err := m.BuildAPI(source).Sync(ctx); err != nil {
		return "", err
	}
	if _, err := m.BuildMCP(source).Sync(ctx); err != nil {
		return "", err
	}

	return "CI passed: lint, test, docs-sync, and build succeeded!", nil
}
