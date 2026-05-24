package server

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/mcp/resources"
	"github.com/0x3639/nom-indexer-go/internal/mcp/tools"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// Implementation name + version reported in the MCP initialize handshake.
// Bump Version when the tool surface meaningfully changes.
const (
	implName    = "nom-indexer-mcp"
	implVersion = "0.1.0"
)

// Deps bundles everything New() needs. Mirrors the router.Deps pattern
// from internal/api — keeps cmd/mcp/main.go declarative.
type Deps struct {
	Repos  *repository.Repositories
	Logger *zap.Logger
	// Version is returned by get_status. cmd/mcp passes the build-time
	// version; tests may leave it empty to use "dev".
	Version string
	// Middlewares are applied to the MCP receiving handler before any
	// tool/resource handler runs. Used to attach the metrics observer
	// (per-tool counter + duration histogram). Order matches the order
	// AddReceivingMiddleware applies them: first middleware wraps last.
	Middlewares []mcp.Middleware
}

// New constructs an mcp.Server populated with every tool + resource
// registered in internal/mcp. The server is transport-agnostic;
// HTTPHandler wraps it for the Streamable HTTP transport.
func New(d Deps) *mcp.Server {
	version := d.Version
	if version == "" {
		version = "dev"
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    implName,
		Version: implVersion,
	}, nil)
	tools.Register(srv, d.Repos, version)
	resources.Register(srv)
	if len(d.Middlewares) > 0 {
		srv.AddReceivingMiddleware(d.Middlewares...)
	}
	return srv
}

// HTTPHandler returns the http.Handler that serves MCP over the
// Streamable HTTP transport. Wraps mcp.NewStreamableHTTPHandler so the
// same server instance is reused across requests (the SDK manages
// per-request sessions).
//
// Transport middleware composition happens at cmd/mcp/main.go — kept out
// of this constructor so tests can build a bare handler without auth/CORS.
func HTTPHandler(srv *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil)
}
