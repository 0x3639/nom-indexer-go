package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// TokenStandardParams targets a token by its ZTS identifier
// (zts1...). ZNN = zts1znnxxxxxxxxxxxxx9z4ulx, QSR = zts1qsrxxxxxxxxxxxxxmrhjll.
type TokenStandardParams struct {
	TokenStandard string `json:"token_standard" jsonschema:"ZTS identifier (zts1...). ZNN and QSR are zts1znnxxxxxxxxxxxxx9z4ulx and zts1qsrxxxxxxxxxxxxxmrhjll respectively."`
}

// TokenHoldersParams targets a token AND paginates the holder list.
type TokenHoldersParams struct {
	TokenStandardParams
	pageParams
}

func registerTokens(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_tokens",
		Description: "List every ZTS token the indexer knows about, ordered by holder_count " +
			"descending then token_standard ascending. Amount fields (total_supply, " +
			"max_supply, total_burned) ship as JSON strings to dodge JavaScript Number " +
			"precision loss (raw int64 values regularly exceed 2^53).",
	}, listTokens(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_token",
		Description: "Return a single ZTS token by its zts1... identifier. Returns an error " +
			"if the indexer has never observed this token.",
	}, getToken(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_token_holders",
		Description: "Richlist for the given ZTS token. Holders are sorted by balance " +
			"descending. Excludes zero balances (the underlying partial index filters " +
			"them). Paginated; default page_size=50, max 200.",
	}, listTokenHolders(repos))
}

func listTokens(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
	// Reuses ListMomentumsParams' pagination embed (sort isn't honored
	// for tokens — the order is fixed). We could declare a dedicated
	// param type, but the pagination shape is identical; keeping one
	// reduces JSON-Schema noise the LLM has to scan.
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Token.List(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromTokens(rows), page.Page, page.PageSize, total))
	}
}

func getToken(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *TokenStandardParams) (*mcp.CallToolResult, *dto.Token, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *TokenStandardParams) (*mcp.CallToolResult, *dto.Token, error) {
		t, err := repos.Token.GetByStandard(ctx, p.TokenStandard)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromToken(t))
	}
}

func listTokenHolders(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *TokenHoldersParams) (*mcp.CallToolResult, *dto.Page, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *TokenHoldersParams) (*mcp.CallToolResult, *dto.Page, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Balance.ListByToken(ctx, p.TokenStandard, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromBalances(rows), page.Page, page.PageSize, total))
	}
}
