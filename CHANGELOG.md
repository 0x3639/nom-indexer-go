# Changelog

All notable changes to this project will be documented in this file.

Versions are not yet released as Git tags; entries here track migration
versions and major doc/code milestones.

## Unreleased

### Added

- Initial mkdocs-material documentation site under `docs/`, deployed
  to GitHub Pages via `.github/workflows/docs-deploy.yml`.
- LLM-friendly flat indexes: `llms.txt` and `llms-full.txt`, with
  generators and CI checks to keep committed copies in sync.
- Per-package `doc.go` files in every `internal/*` package.
- `CONTRIBUTING.md` at the repo root.
- `docs/api/` live REST API reference (OpenAPI 3.1 + Swagger UI) and
  `docs/mcp/` stub tree reserving a slot for the forthcoming MCP server.

### Schema

For per-migration narrative + rationale see
[`docs/migrations/history.md`](docs/migrations/history.md).

| Migration | Adds |
|---|---|
| 001 | Initial 15-table schema. |
| 002 | Performance indexes across all common-query columns. |
| 003 | `wrap_token_requests`, `unwrap_token_requests`. |
| 004 | Bridge finality tracking columns. |
| 005 | `unwrap_token_requests.log_index` widened to `BIGINT`. |
| 006 | Vote dedup + unique constraint. |
| 007 | `token_mints`, `token_burns` event tables. |
| 008 | Bridge configuration tables. |
| 009 | Account flow metrics + balance refresh timestamp. |
| 010 | Daily stat history tables. |
| 011 | `delegations` history table. |
