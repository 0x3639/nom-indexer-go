// Package httpx contains small request/response helpers used by every
// handler in internal/api/handlers. Two responsibilities:
//
//   - respond.go: WriteJSON (Content-Type + status + body) and
//     WriteProblem (RFC 7807 application/problem+json).
//   - query.go: parse pagination + sort query parameters with consistent
//     defaults and bounds (page≥1, 1≤page_size≤200, default 50).
//
// Handlers should not write directly to http.ResponseWriter or read
// raw query values; route everything through this package so the
// response shape stays uniform across the API.
package httpx
