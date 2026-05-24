package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ListStakesParams paginates stakes with an optional include-inactive
// toggle.
type ListStakesParams struct {
	pageParams
	IncludeInactive bool `json:"include_inactive,omitempty" jsonschema:"Include canceled or expired stakes (default false)."`
}

// ListAccountStakesParams scopes the stakes list to one address.
type ListAccountStakesParams struct {
	AddressParams
	pageParams
	IncludeInactive bool `json:"include_inactive,omitempty"`
}

// ListFusionsParams + ListAccountFusionsParams mirror the stakes shape.
type ListFusionsParams struct {
	pageParams
	IncludeInactive bool `json:"include_inactive,omitempty" jsonschema:"Include canceled or expired fusions (default false)."`
}

type ListAccountFusionsParams struct {
	AddressParams
	pageParams
	IncludeInactive bool `json:"include_inactive,omitempty"`
}

func registerStakesFusions(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_stakes",
		Description: "List ZNN stake entries (staked for delegation/voting weight) ordered " +
			"by start_timestamp DESC. Active only by default; include_inactive=true also " +
			"returns canceled/expired stakes. znn_amount ships as a string.",
	}, listStakes(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_account_stakes",
		Description: "Same as list_stakes but scoped to a single address.",
	}, listAccountStakes(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_fusions",
		Description: "List QSR fusion entries (QSR fused to provide plasma for an address) " +
			"ordered by momentum_height DESC. Active only by default; include_inactive=true " +
			"also returns canceled/expired fusions. qsr_amount ships as a string.",
	}, listFusions(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_fusions",
		Description: "Same as list_fusions but scoped to a single address — matches on " +
			"funder OR beneficiary side.",
	}, listAccountFusions(repos))
}

func listStakes(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListStakesParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListStakesParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Stake.List(ctx, !p.IncludeInactive, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromStakes(rows), page.Page, page.PageSize, total))
	}
}

func listAccountStakes(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountStakesParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountStakesParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Stake.ListByAddress(ctx, p.Address, !p.IncludeInactive, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromStakes(rows), page.Page, page.PageSize, total))
	}
}

func listFusions(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListFusionsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListFusionsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Fusion.List(ctx, !p.IncludeInactive, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromFusions(rows), page.Page, page.PageSize, total))
	}
}

func listAccountFusions(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountFusionsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountFusionsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Fusion.ListByAddress(ctx, p.Address, !p.IncludeInactive, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromFusions(rows), page.Page, page.PageSize, total))
	}
}
