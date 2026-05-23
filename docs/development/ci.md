---
title: CI
---

# CI

The project uses [Dagger](https://dagger.io) for the Go pipeline and
GitHub Actions for docs deploys. Both can run locally.

## Dagger pipeline

Module: [`.dagger/main.go`](https://github.com/0x3639/nom-indexer-go/blob/main/.dagger/main.go).

```bash
# Full CI: lint + test + build.
dagger call ci --source .

# Single steps:
dagger call lint --source .
dagger call test --source .
dagger call build --source .

# Publish to GHCR (requires auth):
dagger call publish --source . --tag <version>
```

`dagger call ci` is what you run before opening a PR. It chains:

1. `lint` — `golangci-lint v2` with `.golangci.yml`.
2. `test` — `go test ./...` (unit tests only — integration tests need
   a Postgres that Dagger isn't wiring today).
3. `build` — multi-stage Docker build that mirrors the production
   Dockerfile.

## When Dagger runs out of disk

```bash
docker stop dagger-engine-* && docker rm dagger-engine-* && docker volume prune -f
```

## GitHub Actions

| Workflow | Trigger | Purpose |
|---|---|---|
| [`ci.yml`](https://github.com/0x3639/nom-indexer-go/blob/main/.github/workflows/ci.yml) | push / PR | Run the Dagger `ci` pipeline; publish GHCR images on `main` and version tags. |
| [`docs-deploy.yml`](https://github.com/0x3639/nom-indexer-go/blob/main/.github/workflows/docs-deploy.yml) | push to `main` | Build `mkdocs --strict` and publish to GitHub Pages. |
| [`docs-pr-preview.yml`](https://github.com/0x3639/nom-indexer-go/blob/main/.github/workflows/docs-pr-preview.yml) | PR touching docs files | Same strict build, no deploy. Catches broken links before merge. |

The PR workflow also runs `gen-llms-txt.py --check` and
`gen-llms-full.py --check`, which fail if the committed indexes are
out of sync with `mkdocs.yml` / `docs/`.

## Adding a CI step

For Go code: add a method on the Dagger `m *NomIndexerGo` type in
`.dagger/main.go`, then add a call in the `Ci` orchestrator. Dagger
auto-regenerates the SDK code on the next run.

For docs: edit the existing GitHub Actions workflows. Don't add new
workflows for docs unless you have a strong reason — the two we
have already cover the deploy and the PR strict-build paths.

## Local CI smoke test

Before pushing:

```bash
# Lint + tests + build (the Go side):
GOWORK=off go build ./...
GOWORK=off go test ./...
golangci-lint run ./...

# Docs:
.venv-docs/bin/python scripts/docs/gen-llms-txt.py --check
.venv-docs/bin/python scripts/docs/gen-llms-full.py --check
.venv-docs/bin/mkdocs build --strict
```

All five must pass. If `--check` fails, run the generator without
`--check`, commit the diff, then re-check.

## GHCR publish

The published image is `ghcr.io/0x3639/nom-indexer-go:<tag>`. Tag
selection and signing are governed by the Dagger publish step. Today
this is invoked manually; a release workflow is plausible follow-up.
