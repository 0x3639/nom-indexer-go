package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ListSentinelsParams paginates the sentinel set with an optional
// retired-sentinels toggle.
type ListSentinelsParams struct {
	pageParams
	IncludeInactive bool `json:"include_inactive,omitempty" jsonschema:"Include retired (inactive) sentinels (default false)."`
}

func registerSentinels(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_sentinels",
		Description: "List Zenon sentinels (secondary network nodes, lighter-stake than " +
			"pillars) ordered by registration_timestamp DESC (newest first). Active " +
			"sentinels only by default; pass include_inactive=true to include retired ones.",
	}, listSentinels(repos))
}

func listSentinels(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListSentinelsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListSentinelsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		activeOnly := !p.IncludeInactive
		rows, total, err := repos.Sentinel.List(ctx, activeOnly, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromSentinels(rows), page.Page, page.PageSize, total))
	}
}
