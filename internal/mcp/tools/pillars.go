package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ListPillarsParams paginates the validator set with an optional
// revoked-pillars toggle.
type ListPillarsParams struct {
	pageParams
	IncludeRevoked bool `json:"include_revoked,omitempty" jsonschema:"Include revoked pillars (default false)."`
}

// PillarNameParams targets a pillar by name. Pillar names are unique
// in Zenon.
type PillarNameParams struct {
	Name string `json:"name" jsonschema:"Pillar name (case-sensitive, unique). Example: SultanOfStaking."`
}

// ListPillarDelegatorsParams paginates an address's delegators by
// pillar name. The tool resolves name → owner_address internally.
type ListPillarDelegatorsParams struct {
	PillarNameParams
	pageParams
}

func registerPillars(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_pillars",
		Description: "List Zenon pillars (validators) ordered by rank ascending. " +
			"Excludes revoked pillars by default; pass include_revoked=true to include them. " +
			"Weight and slot_cost_qsr are stringified ints (raw int64 QSR amounts).",
	}, listPillars(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_pillar_by_name",
		Description: "Return a single pillar by its name. Pillar names are unique in Zenon. " +
			"Returns an error if no pillar with that name exists.",
	}, getPillarByName(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_pillar_delegators",
		Description: "List addresses currently delegating to the named pillar, sorted by " +
			"delegation_start_timestamp ASC (longest-tenured first). Looks up name → owner " +
			"first; returns 404 (via tool error) if the pillar name is unknown.",
	}, listPillarDelegators(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_pillar_voting_history",
		Description: "Complete voting record for one named pillar across every Accelerator-Z " +
			"project AND phase, with project + phase names joined server-side and vote codes " +
			"translated to \"yes\" / \"no\" / \"abstain\". Returns yes/no/abstain totals plus " +
			"the per-vote entries ordered momentum_timestamp DESC (newest first). One call " +
			"replaces enumerate-projects + page-through-list_project_votes + filter-by-pillar.",
	}, getPillarVotingHistory(repos))
}

func listPillars(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListPillarsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListPillarsParams) (*mcp.CallToolResult, any, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Pillar.List(ctx, p.IncludeRevoked, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromPillars(rows), page.Page, page.PageSize, total))
	}
}

func getPillarByName(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *PillarNameParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *PillarNameParams) (*mcp.CallToolResult, any, error) {
		pillar, err := repos.Pillar.GetByName(ctx, p.Name)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromPillar(pillar))
	}
}

func listPillarDelegators(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListPillarDelegatorsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListPillarDelegatorsParams) (*mcp.CallToolResult, any, error) {
		// Mirror the REST handler: resolve name → owner first so a bad
		// pillar name surfaces as a clean 404, not an empty list.
		pillar, err := repos.Pillar.GetByName(ctx, p.Name)
		if err != nil {
			return nil, nil, err
		}
		page := pagination(p.pageParams)
		rows, total, err := repos.Pillar.ListDelegators(ctx, pillar.OwnerAddress, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromPillarDelegators(rows), page.Page, page.PageSize, total))
	}
}

func getPillarVotingHistory(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *PillarNameParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *PillarNameParams) (*mcp.CallToolResult, any, error) {
		row, err := repos.Vote.PillarVotingHistory(ctx, p.Name)
		if err != nil {
			return nil, nil, err
		}
		raws := make([]dto.RawPillarVote, 0, len(row.Votes))
		for _, v := range row.Votes {
			raws = append(raws, dto.RawPillarVote{
				VotingID:          v.VotingID,
				Vote:              v.Vote,
				MomentumHeight:    v.MomentumHeight,
				MomentumTimestamp: v.MomentumTimestamp,
				ProjectID:         v.ProjectID,
				PhaseID:           v.PhaseID,
				ProjectName:       v.ProjectName,
				PhaseName:         v.PhaseName,
			})
		}
		return jsonResult(dto.FromPillarVotingHistory(row.PillarName, row.PillarOwner, raws))
	}
}
