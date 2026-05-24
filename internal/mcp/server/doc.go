// Package server wraps the modelcontextprotocol/go-sdk mcp.Server with
// the nom-indexer-go-specific lifecycle: HS256 JWT bearer auth on the
// HTTP transport, per-subject rate limiting, Prometheus metrics, and
// CORS allowlist.
//
// The package exposes two top-level concerns:
//
//   - New builds an *mcp.Server populated with every tool registered in
//     internal/mcp/tools/register.go. The server is transport-agnostic —
//     the same instance can be served over Streamable HTTP or stdio.
//
//   - HTTPHandler wraps mcp.NewStreamableHTTPHandler. cmd/mcp composes
//     transport-level middleware (CORS, auth, rate limit) around it and
//     passes MCP receiving middleware (metrics) into New.
//
// Tool wiring lives in internal/mcp/tools (one file per domain). The
// server package itself never knows what tools exist — it just runs
// whatever Register() puts on the mcp.Server.
package server
