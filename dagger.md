# Dagger Integration Roadmap

This document outlines the plan to integrate [Dagger](https://dagger.io/) into nom-indexer-go for CI/CD automation.

## Goals

- **Local dev workflow** - Run builds, tests, and linting with consistent commands
- **CI/CD automation** - Automated pipelines on GitHub Actions
- **Publish to ghcr.io** - Push container images to GitHub Container Registry
- **Keep existing Dockerfiles** - Dagger will orchestrate them, not replace them

## What is Dagger?

Dagger is a platform for building CI/CD pipelines as code using Go (or Python/TypeScript). Benefits:

| Benefit | Description |
|---------|-------------|
| **Portable** | Same pipeline runs locally and in CI |
| **Type-safe** | Full IDE support, type checking, autocompletion |
| **Caching** | Automatic layer caching across runs |
| **No vendor lock-in** | Works with any CI system |
| **Containerized** | Consistent execution everywhere |

## Implementation Steps

### Step 1: Install Dagger CLI

```bash
# macOS
brew install dagger/tap/dagger

# Or via curl
curl -fsSL https://dl.dagger.io/dagger/install.sh | sh

# Verify installation
dagger version
```

### Step 2: Initialize Dagger Module

```bash
dagger init --sdk=go --name=nom-indexer
```

This creates a `.dagger/` directory with:
- `dagger.json` - Module configuration
- `go.mod` / `go.sum` - Go module files
- `main.go` - Pipeline functions (to be implemented)

### Step 3: Implement Pipeline Functions

Create `.dagger/main.go` with the following functions:

| Function | Description | Command |
|----------|-------------|---------|
| `Test()` | Run `go test ./...` | `dagger call test --source=.` |
| `Lint()` | Run golangci-lint | `dagger call lint --source=.` |
| `Container()` | Build Docker image from Dockerfile | `dagger call container --source=.` |
| `Publish(tag)` | Push to ghcr.io | `dagger call publish --source=. --tag=latest` |
| `CI()` | Full pipeline: lint → test → build | `dagger call ci --source=.` |

#### Example Implementation

```go
package main

import (
    "context"
    "dagger/nom-indexer/internal/dagger"
)

type NomIndexer struct{}

// Build the indexer using existing Dockerfile
func (m *NomIndexer) Container(ctx context.Context, source *dagger.Directory) *dagger.Container {
    return dag.Container().
        Build(source, dagger.ContainerBuildOpts{
            Dockerfile: "Dockerfile",
        })
}

// Run tests
func (m *NomIndexer) Test(ctx context.Context, source *dagger.Directory) (string, error) {
    return dag.Container().
        From("golang:1.24-alpine").
        WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
        WithDirectory("/app", source).
        WithWorkdir("/app").
        WithExec([]string{"go", "test", "./..."}).
        Stdout(ctx)
}

// Run linter
func (m *NomIndexer) Lint(ctx context.Context, source *dagger.Directory) (string, error) {
    return dag.Container().
        From("golangci/golangci-lint:latest").
        WithDirectory("/app", source).
        WithWorkdir("/app").
        WithExec([]string{"golangci-lint", "run", "./..."}).
        Stdout(ctx)
}

// Publish to GitHub Container Registry
func (m *NomIndexer) Publish(ctx context.Context, source *dagger.Directory, tag string) (string, error) {
    return m.Container(ctx, source).
        Publish(ctx, "ghcr.io/0x3639/nom-indexer-go:" + tag)
}

// Full CI pipeline
func (m *NomIndexer) CI(ctx context.Context, source *dagger.Directory) (string, error) {
    // Run lint
    if _, err := m.Lint(ctx, source); err != nil {
        return "", err
    }
    // Run tests
    if _, err := m.Test(ctx, source); err != nil {
        return "", err
    }
    // Build container (validates Dockerfile)
    _ = m.Container(ctx, source)
    return "CI passed!", nil
}
```

### Step 4: Create GitHub Actions Workflow

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: ci --source=.

  publish:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    needs: ci
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: dagger/dagger-for-github@v7
        with:
          verb: call
          args: publish --source=. --tag=${{ github.sha }}
```

## Usage

### Local Development

```bash
# Run tests
dagger call test --source=.

# Run linter
dagger call lint --source=.

# Build container image
dagger call container --source=.

# Run full CI pipeline locally (same as GitHub Actions)
dagger call ci --source=.

# Publish to ghcr.io (requires authentication)
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
dagger call publish --source=. --tag=latest
```

### CI/CD (Automatic)

| Trigger | Action |
|---------|--------|
| Push to PR | Runs lint + test |
| Merge to main | Builds and publishes to ghcr.io |

## Files Overview

### Files to Create

| File | Purpose |
|------|---------|
| `.dagger/main.go` | Pipeline functions in Go |
| `.dagger/dagger.json` | Module configuration (auto-generated) |
| `.dagger/go.mod` | Go module (auto-generated) |
| `.github/workflows/ci.yml` | GitHub Actions workflow |

### Existing Files (Unchanged)

| File | Purpose |
|------|---------|
| `Dockerfile` | Used by Dagger for container builds |
| `Dockerfile.migrate` | Migration tool (can add Dagger function later) |
| `docker-compose.yml` | Local development with postgres |

## Resources

- [Dagger Documentation](https://docs.dagger.io/)
- [Dagger Go SDK](https://docs.dagger.io/manuals/developer/go)
- [Dagger for GitHub Actions](https://docs.dagger.io/integrations/github-actions)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
