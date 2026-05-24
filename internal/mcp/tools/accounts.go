package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// AddressParams is the common shape for tools keyed by a single
// Zenon address. Pulled into its own type so the LLM sees a
// consistent input schema across the account-scoped tools.
type AddressParams struct {
	Address string `json:"address" jsonschema:"Zenon address (z1q...). Required."`
}

func registerAccounts(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_account",
		Description: "Return an account's profile: lifetime ZNN/QSR flow metrics, current " +
			"delegation, first/last activity timestamps. Returns an error if the indexer " +
			"has never observed this address.",
	}, getAccount(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_balances",
		Description: "Return every (token_standard, balance) row for the given address, " +
			"sorted by balance descending. Not paginated — accounts typically hold a handful " +
			"of tokens. Empty result is returned as {data: []} (not an error).",
	}, listAccountBalances(repos))
}

func getAccount(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *AddressParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *AddressParams) (*mcp.CallToolResult, any, error) {
		a, err := repos.Account.GetByAddress(ctx, p.Address)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromAccount(a))
	}
}

// listAccountBalancesResult wraps the unpaginated balances slice in a
// {data: [...]} envelope. Matches the REST shape so AI consumers see a
// consistent payload across both transports.
type listAccountBalancesResult struct {
	Data []*dto.Balance `json:"data"`
}

func listAccountBalances(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *AddressParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *AddressParams) (*mcp.CallToolResult, any, error) {
		rows, err := repos.Balance.ListByAddress(ctx, p.Address)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(&listAccountBalancesResult{Data: dto.FromBalances(rows)})
	}
}
