---
title: Development setup
---

# Development setup

How to get a local checkout building and tests running.

## Prerequisites

- Go **1.25+**. `go.mod` declares 1.25 (the prometheus + kin-openapi
  upgrades pulled the floor up from 1.24); older toolchains will
  refuse to compile.
- A working C toolchain. CGO is required for `secp256k1`:
    - macOS: Xcode command-line tools (`xcode-select --install`).
    - Linux: `gcc` + `libc-dev` (usually `apt install build-essential`).
- Docker Engine + Compose v2 for the integration test DB + the
  reference Docker Compose stack.
- Postgres client (`psql`) for ad-hoc queries — optional, but most
  recipes use it.

## First build

```bash
git clone https://github.com/0x3639/nom-indexer-go.git
cd nom-indexer-go

# GOWORK=off is required if you have a parent go.work that doesn't
# include this module.
GOWORK=off go build ./...
```

Successful output: `internal/...` and `cmd/indexer` + `cmd/backfill` +
`scripts/...` all compile.

## SDK dependency

`go.mod` uses upstream `github.com/0x3639/znn-sdk-go` directly. There
is no local `vendor-sdk/` directory and no `replace` directive in the
current checkout.

A historical accelerator-project JSON decoding panic was fixed in the
SDK before the current version. If that panic returns after an SDK
bump, treat it as an upstream regression: pin or roll back the SDK
version and document the reason in
[`docs/reference/known-issues.md`](../reference/known-issues.md).

## Running tests

```bash
# Unit tests (no DB required).
GOWORK=off go test ./...

# Integration tests (need a running Postgres on localhost:5432).
docker compose up -d postgres
docker exec nom-indexer-postgres psql -U postgres -c 'CREATE DATABASE nom_indexer_test;'

TEST_DATABASE_URL='postgres://postgres:<your-pw>@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./...
```

The `TestMain` in
[`internal/repository/integration_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_test.go)
runs all migrations on the test DB on every invocation; per-test
TRUNCATE keeps a clean slate.

See [`docs/testing/strategy.md`](../testing/strategy.md) for more.

## Linting

```bash
# Install golangci-lint v2 (one-time):
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Run:
golangci-lint run ./...
```

Lint config is `.golangci.yml`. Import ordering follows `gci`:

1. Standard library
2. Third-party
3. Local module (`github.com/0x3639/nom-indexer-go/...`)

## Live indexer for development

```bash
docker compose up -d --build
docker logs nom-indexer -f
```

Rebuilds the image with your local code on every `up -d --build`.
Useful for end-to-end tests against a real test node.

## Editor setup

- **gopls** with the repo as a workspace root just works.
- Recommended `.editorconfig` lives in repo root (not present today —
  consider adding).

## Docs site preview

The docs are mkdocs-material under [`docs/`](https://github.com/0x3639/nom-indexer-go/tree/main/docs).

```bash
# One-time setup of a Python venv for the docs (PEP 668 systems).
python3 -m venv .venv-docs
.venv-docs/bin/pip install --quiet \
  mkdocs-material mkdocs-mermaid2-plugin \
  mkdocs-include-markdown-plugin mkdocs-swagger-ui-tag \
  pymdown-extensions pyyaml

# Serve locally at http://127.0.0.1:8000:
.venv-docs/bin/mkdocs serve

# Strict build (what CI runs):
.venv-docs/bin/mkdocs build --strict

# Regenerate the LLM indexes:
.venv-docs/bin/python scripts/docs/gen-llms-txt.py
.venv-docs/bin/python scripts/docs/gen-llms-full.py
```

See [`docs.md`](docs.md) for the doc authoring workflow.
