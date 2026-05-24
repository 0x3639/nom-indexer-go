package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// Register wires every tool onto srv. This is the single source of
// truth for the tool catalog — internal/mcp/server/New() calls it
// once at boot.
//
// Tools are grouped by domain. Each registerXxx helper lives in the
// matching domain file (status.go, momentums.go, ...) and calls
// mcp.AddTool one or more times. Keeping the helpers per-file makes
// the catalog easy to scan: one file, one domain, one mental model.
func Register(srv *mcp.Server, repos *repository.Repositories) {
	registerStatus(srv, repos)
	registerMomentums(srv, repos)
	registerAccounts(srv, repos)
	registerTokens(srv, repos)
}
