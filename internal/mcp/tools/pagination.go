package tools

import (
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
)

// pageParams mirrors the REST API's pagination contract for tools that
// list collections. Embed in a tool's argument struct to inherit the
// defaults (page=1, page_size=50, max 200) without per-tool wiring.
type pageParams struct {
	Page     int `json:"page,omitempty" jsonschema:"1-based page number (default 1, clamped to >=1)"`
	PageSize int `json:"page_size,omitempty" jsonschema:"Items per page (default 50, max 200)"`
}

// sortParam optionally narrows ordering on tools that accept it.
type sortParam struct {
	Sort string `json:"sort,omitempty" jsonschema:"asc | desc (default: per-tool)"`
}

// pagination returns the clamped (page, page_size) tuple plus an
// httpx.Pagination convenience for converting to SQL Offset(). Defaults
// match internal/api/httpx so the MCP wire shape stays identical to
// the REST envelope.
func pagination(p pageParams) httpx.Pagination {
	page := p.Page
	if page < 1 {
		page = 1
	}
	size := p.PageSize
	if size < 1 {
		size = httpx.DefaultPageSize
	}
	if size > httpx.MaxPageSize {
		size = httpx.MaxPageSize
	}
	return httpx.Pagination{Page: page, PageSize: size}
}

// sortDirection clamps to "asc" / "desc"; anything else returns the
// caller's default. Mirrors httpx.ParseSort semantics so tool +
// REST callers experience the same behavior.
func sortDirection(s sortParam, def string) string {
	switch s.Sort {
	case "asc", "desc":
		return s.Sort
	default:
		return def
	}
}
