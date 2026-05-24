package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// HashParams targets a specific account-block by hash.
type HashParams struct {
	Hash string `json:"hash" jsonschema:"64-char lowercase hex hash of the account-block."`
}

// ListAccountTransactionsParams paginates an address's transaction
// history (account_blocks where the address is sender or recipient).
type ListAccountTransactionsParams struct {
	AddressParams
	pageParams
	sortParam
}

func registerAccountBlocks(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_blocks",
		Description: "List account_blocks (transactions in Zenon's dual-ledger model). " +
			"Default sort=desc by momentum_height. pagination.total is approximate (sourced " +
			"from pg_class.reltuples; refreshed by ANALYZE/autovacuum). For deep history " +
			"prefer list_account_transactions scoped to a specific address — far cheaper.",
	}, listAccountBlocks(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_account_block",
		Description: "Return one account-block by its hash. An account-block is either a " +
			"send (block_type 1..4) or a receive (5..8); a complete transfer is the matched " +
			"(send, receive) pair linked via paired_account_block.",
	}, getAccountBlock(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_transactions",
		Description: "List account_blocks involving the given address as sender or " +
			"recipient, ordered by momentum_height (default desc). Exact pagination.total. " +
			"Paginated; default page_size=50, max 200.",
	}, listAccountTransactions(repos))
}

func listAccountBlocks(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.AccountBlock.List(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
			Sort:   sortDirection(p.sortParam, "desc"),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromAccountBlocks(rows), page.Page, page.PageSize, total))
	}
}

func getAccountBlock(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *HashParams) (*mcp.CallToolResult, *dto.AccountBlock, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *HashParams) (*mcp.CallToolResult, *dto.AccountBlock, error) {
		ab, err := repos.AccountBlock.GetByHash(ctx, p.Hash)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromAccountBlock(ab))
	}
}

func listAccountTransactions(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountTransactionsParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountTransactionsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.AccountBlock.ListByAddress(ctx, p.Address, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
			Sort:   sortDirection(p.sortParam, "desc"),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromAccountBlocks(rows), page.Page, page.PageSize, total))
	}
}
