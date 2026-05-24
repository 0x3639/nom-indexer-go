package httpx

import (
	"net/http"
	"strconv"
)

// Pagination defaults applied across the API. Documented in
// docs/api/pagination.md.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Pagination is the parsed (page, page_size) tuple plus a convenience
// Offset() for SQL.
type Pagination struct {
	Page     int
	PageSize int
}

// Offset returns the SQL OFFSET equivalent for this page.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// ParsePagination reads ?page and ?page_size from the request and
// applies the API defaults + bounds (page≥1, 1≤page_size≤MaxPageSize).
// Out-of-range values are clamped silently — the API never 400s on
// pagination misuse.
func ParsePagination(r *http.Request) Pagination {
	p := Pagination{Page: 1, PageSize: DefaultPageSize}

	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > MaxPageSize {
				n = MaxPageSize
			}
			p.PageSize = n
		}
	}
	return p
}

// ParseSort reads the ?sort query parameter. Returns "asc" or "desc";
// any other value (or absence) returns the default. Used by collection
// endpoints that document a single sortable column per route.
func ParseSort(r *http.Request, def string) string {
	switch r.URL.Query().Get("sort") {
	case "asc":
		return "asc"
	case "desc":
		return "desc"
	default:
		return def
	}
}
