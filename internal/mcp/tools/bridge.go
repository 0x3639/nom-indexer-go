package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ListAccountBridgeParams paginates a per-address wrap/unwrap list.
// Address filter matches on `to_address` (destination of the
// cross-chain transfer), mirroring the REST endpoint.
type ListAccountBridgeParams struct {
	AddressParams
	pageParams
}

func registerBridge(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_bridge_wraps",
		Description: "List wrap requests (Zenon → external chain transfers) ordered by " +
			"creation_momentum_height DESC. amount + fee ship as stringified ints.",
	}, listBridgeWraps(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_bridge_unwraps",
		Description: "List unwrap requests (external chain → Zenon transfers) ordered by " +
			"registration_momentum_height DESC.",
	}, listBridgeUnwraps(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_bridge_wraps",
		Description: "List wraps whose `to_address` (external destination) matches the " +
			"given address. Most useful keyed by an EVM address (0x...).",
	}, listAccountBridgeWraps(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_bridge_unwraps",
		Description: "List unwraps whose `to_address` (Zenon destination) matches the " +
			"given address.",
	}, listAccountBridgeUnwraps(repos))
}

func listBridgeWraps(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Bridge.ListWraps(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromWrapTokenRequests(rows), page.Page, page.PageSize, total))
	}
}

func listBridgeUnwraps(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Bridge.ListUnwraps(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromUnwrapTokenRequests(rows), page.Page, page.PageSize, total))
	}
}

func listAccountBridgeWraps(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountBridgeParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountBridgeParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Bridge.ListWrapsByAddress(ctx, p.Address, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromWrapTokenRequests(rows), page.Page, page.PageSize, total))
	}
}

func listAccountBridgeUnwraps(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountBridgeParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountBridgeParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Bridge.ListUnwrapsByAddress(ctx, p.Address, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromUnwrapTokenRequests(rows), page.Page, page.PageSize, total))
	}
}
