package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ListAccountRewardsParams paginates the per-event reward history for
// one address.
type ListAccountRewardsParams struct {
	AddressParams
	pageParams
}

func registerRewards(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_account_rewards",
		Description: "Per-event reward history for an address, ordered by momentum_height " +
			"DESC. Each row is one reward-receive transaction; reward_type is one of " +
			"Pillar, Sentinel, Stake, Delegation, Liquidity, Unknown. Amounts ship as " +
			"strings.",
	}, listAccountRewards(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_account_cumulative_rewards",
		Description: "Lifetime cumulative rewards for an address: one row per " +
			"(reward_type, token_standard) bucket. Not paginated — typically a handful of " +
			"rows per address. Returns {data: []} for addresses with no rewards.",
	}, getAccountCumulativeRewards(repos))
}

func listAccountRewards(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListAccountRewardsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListAccountRewardsParams) (*mcp.CallToolResult, any, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Reward.HistoryByAddress(ctx, p.Address, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromRewardTransactions(rows), page.Page, page.PageSize, total))
	}
}

// cumulativeRewardsResult wraps the unpaginated slice in {data: [...]}.
type cumulativeRewardsResult struct {
	Data []*dto.CumulativeReward `json:"data"`
}

func getAccountCumulativeRewards(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *AddressParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *AddressParams) (*mcp.CallToolResult, any, error) {
		rows, err := repos.Reward.CumulativeByAddress(ctx, p.Address)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(&cumulativeRewardsResult{Data: dto.FromCumulativeRewards(rows)})
	}
}
