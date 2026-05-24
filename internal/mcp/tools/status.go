package tools

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// nowFn is overridable for tests that want to pin indexer_lag_seconds.
// Production callers use time.Now via the package default.
var nowFn = time.Now

// GetStatusParams takes no arguments. Defined as a named struct so the
// SDK can derive a JSON Schema (an empty {}).
type GetStatusParams struct{}

func registerStatus(srv *mcp.Server, repos *repository.Repositories, version string) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_status",
		Description: "Return the indexer's current sync state — latest momentum height, its " +
			"Unix timestamp, and indexer_lag_seconds (server clock minus latest momentum " +
			"timestamp). Computed entirely from the database; does not contact the Zenon " +
			"node. An indexer_lag_seconds value larger than ~30 indicates the indexer is " +
			"falling behind the chain head. Returns latest_height=0 on an empty DB.",
	}, getStatus(repos, version))
}

func getStatus(repos *repository.Repositories, version string) func(context.Context, *mcp.CallToolRequest, *GetStatusParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *GetStatusParams) (*mcp.CallToolResult, any, error) {
		m, err := repos.Momentum.GetLatest(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return jsonResult(&dto.Status{Version: version})
		}
		if err != nil {
			return nil, nil, err
		}
		lag := nowFn().Unix() - m.Timestamp
		if lag < 0 {
			lag = 0
		}
		return jsonResult(&dto.Status{
			LatestHeight:      m.Height,
			LatestTimestamp:   m.Timestamp,
			IndexerLagSeconds: lag,
			Version:           version,
		})
	}
}
