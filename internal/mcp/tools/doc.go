// Package tools holds every MCP tool the server exposes.
//
// Conventions:
//
//   - One file per logical domain (status.go, momentums.go,
//     accounts.go, ...). New domains get a new file; the existing
//     ones never grow unboundedly.
//
//   - Each tool is one exported function returning either an
//     mcp.Tool struct + handler closure, OR registered directly via
//     mcp.AddTool inside Register().
//
//   - Tools call internal/repository directly (no HTTP, no DTO
//     conversion in the middle) and return dto.* values. The SDK
//     marshals the result to JSON for the wire.
//
//   - register.go is the single source of truth for which tools
//     exist. Adding a tool means: write the handler in the
//     domain file, then add one mcp.AddTool call in Register().
package tools
