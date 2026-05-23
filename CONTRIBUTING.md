# Contributing

Thanks for considering a contribution to `nom-indexer-go`. This document
collects the conventions that aren't enforced by lint or CI.

For deeper context, see [`docs/`](docs/) — particularly
[`docs/development/setup.md`](docs/development/setup.md),
[`docs/development/project-layout.md`](docs/development/project-layout.md),
and [`docs/development/coding-style.md`](docs/development/coding-style.md).

## Before you start

- Open an issue first for non-trivial changes so we can align on scope.
- One-line typo fixes can skip the issue.
- If you're touching the schema, read
  [`docs/schema/conventions.md`](docs/schema/conventions.md) and
  [`docs/migrations/guide.md`](docs/migrations/guide.md) first.

## Branches and commits

- Branch from `main`. Name it `<author>/<short-description>` or
  `<topic>/<short-description>`.
- Subject line: imperative, ≤ 70 chars. Body explains *why*, not
  *what*.
- Squash before merge if your branch has WIP commits.
- Sign off your commits if your fork is under a CLA flow.

## Required local checks

Before opening a PR:

```bash
GOWORK=off go build ./...
GOWORK=off go test ./...
golangci-lint run ./...

# If you touched docs/, mkdocs.yml, scripts/docs/, or README/CLAUDE/CONTRIBUTING:
.venv-docs/bin/python scripts/docs/gen-llms-txt.py
.venv-docs/bin/python scripts/docs/gen-llms-full.py
.venv-docs/bin/mkdocs build --strict
```

CI runs the same flow. PRs cannot merge if anything fails.

## Pull request checklist

- [ ] Builds with `GOWORK=off go build ./...`.
- [ ] Unit tests pass.
- [ ] Integration tests pass if you touched repository / migration / schema code.
- [ ] Linter clean.
- [ ] Docs updated when behavior changes:
    - [ ] Schema docs (`docs/schema/<table>.md`) for new columns or
      tables.
    - [ ] Indexing docs (`docs/indexing/<contract>-contract.md`) for
      new contract behavior.
    - [ ] Config reference (`docs/config/reference.md`) for new env
      vars or YAML keys.
    - [ ] Migration history (`docs/migrations/history.md`) for new
      migrations.
- [ ] `llms.txt` and `llms-full.txt` regenerated and committed if you
  added or renamed pages.
- [ ] `mkdocs build --strict` passes locally.

## What we look for in review

- **Correctness over cleverness.** Boring patterns that mirror existing
  code are usually right.
- **Idempotent writes.** Every repository insert/upsert must be safe
  to re-run. See
  [`docs/schema/conventions.md`](docs/schema/conventions.md#batch-writes-and-idempotency).
- **Tests that verify behavior, not just coverage.** Table-driven for
  pure helpers; round-trip for repositories.
- **No silent failures.** Wrap errors with `%w`, log structured fields,
  return the error so the sync loop can retry.

## Issue reports

Open an issue for:

- Bugs — include the indexer's recent log output (the relevant ~50 lines)
  and the matching `SELECT` from any affected table.
- Feature requests — describe the use case before the proposed solution.
- Documentation gaps — link the page that needs updating.

## License

Contributions are licensed under MIT, matching the project license.
