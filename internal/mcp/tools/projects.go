package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// ProjectIDParams targets one Accelerator-Z project by ID.
type ProjectIDParams struct {
	ID string `json:"id" jsonschema:"Accelerator-Z project ID (64-char hex)."`
}

// ListProjectVotesParams paginates a project's votes.
type ListProjectVotesParams struct {
	ProjectIDParams
	pageParams
}

func registerProjects(srv *mcp.Server, repos *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_projects",
		Description: "List Accelerator-Z projects ordered by creation_timestamp DESC " +
			"(newest first). znn_funds_needed and qsr_funds_needed ship as strings.",
	}, listProjects(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_project",
		Description: "Return one Accelerator-Z project by its ID. Includes vote tallies " +
			"(yes_votes, no_votes, total_votes) and funding requested.",
	}, getProject(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_project_phases",
		Description: "List every phase of the given project in creation order ASC " +
			"(phase 1 first). Not paginated.",
	}, listProjectPhases(repos))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_project_votes",
		Description: "List pillar votes targeting the given project (or any of its phases), " +
			"ordered by momentum_height DESC. vote=0 yes, 1 no, 2 abstain.",
	}, listProjectVotes(repos))
}

func listProjects(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListMomentumsParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListMomentumsParams) (*mcp.CallToolResult, any, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Project.List(ctx, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromProjects(rows), page.Page, page.PageSize, total))
	}
}

func getProject(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ProjectIDParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ProjectIDParams) (*mcp.CallToolResult, any, error) {
		pr, err := repos.Project.GetByID(ctx, p.ID)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.FromProject(pr))
	}
}

// projectPhasesResult wraps the unpaginated slice in {data: [...]}.
type projectPhasesResult struct {
	Data []*dto.ProjectPhase `json:"data"`
}

func listProjectPhases(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ProjectIDParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ProjectIDParams) (*mcp.CallToolResult, any, error) {
		rows, err := repos.ProjectPhase.ListByProject(ctx, p.ID)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(&projectPhasesResult{Data: dto.FromProjectPhases(rows)})
	}
}

func listProjectVotes(repos *repository.Repositories) func(context.Context, *mcp.CallToolRequest, *ListProjectVotesParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p *ListProjectVotesParams) (*mcp.CallToolResult, any, error) {
		page := pagination(p.pageParams)
		rows, total, err := repos.Vote.ListByProject(ctx, p.ID, repository.ListOpts{
			Limit:  page.PageSize,
			Offset: page.Offset(),
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(dto.NewPage(dto.FromVotes(rows), page.Page, page.PageSize, total))
	}
}
