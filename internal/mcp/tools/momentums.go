package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// GetMomentumByHeightParams targets a specific block by height.
type GetMomentumByHeightParams struct {
	Height uint64 `json:"height" jsonschema:"Block height (uint64). Returns an error if no momentum exists at this height."`
}

// ListMomentumsParams paginates the full momentums table ordered by
// height. Defaults to newest-first (sort=desc).
type ListMomentumsParams struct {
	pageParams
	sortParam
}

func registerMomentums(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_momentum_by_height",
		Description: "Return the momentum (block header) at the given height. Heights are " +
			"dense integers starting at 1; the chain commits a new momentum roughly every " +
			"10 seconds. Returns an error if no momentum exists at that height (e.g., " +
			"above the current chain tip or in a backfill gap).",
	}, getMomentumByHeight(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_latest_momentum",
		Description: "Return the highest-height momentum the indexer has processed. Useful " +
			"as a chain-tip pointer or to bound a from_height for paginated reads.",
	}, getLatestMomentum(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_momentums",
		Description: "List momentums ordered by height. Default sort=desc (newest first). " +
			"page is 1-based; page_size defaults to 50 and caps at 200. " +
			"pagination.total is an upper bound (MAX(height) — exact when the chain has no " +
			"backfill gaps). For deep pagination prefer get_momentum_by_height with a " +
			"specific height — large offsets are slow on the 13M-row momentums table.",
	}, listMomentums(repos))
}

func getMomentumByHeight(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *GetMomentumByHeightParams) (*mcp.CallToolResult, *dto.Momentum, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *GetMomentumByHeightParams) (*mcp.CallToolResult, *dto.Momentum, error) {
		m, err := repos.Momentum.GetByHeight(ctx, p.Height)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromMomentum(m))
	}
}

func getLatestMomentum(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *struct{}) (*mcp.CallToolResult, *dto.Momentum, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, *dto.Momentum, error) {
		m, err := repos.Momentum.GetLatest(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromMomentum(m))
	}
}

func listMomentums(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Momentum.List(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
			Sort:   sortDirection(p.sortParam, "desc"),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromMomentums(rows), page.Page, page.PageSize, total))
	}
}
