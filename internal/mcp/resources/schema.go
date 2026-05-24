// Package resources owns the MCP resources exposed by the server.
// Resources differ from tools in that they are read-only data the LLM
// can pull as context — useful for grounding tool selection in the
// underlying data model.
//
// v1 ships one resource: schema://overview, a compact JSON catalog
// of every table the indexer fills. LLMs reach for it before choosing
// a tool when the user's question doesn't obviously map to one.
package resources

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SchemaOverviewURI is the canonical URI for the schema overview resource.
// Hand to clients so they can request it explicitly via resources/read.
const SchemaOverviewURI = "schema://overview"

// table is one row of the schema overview catalog. The MCP tools that
// read a table are listed in `Tools` so an LLM can map a table back to
// the tool surface without re-reading the tool catalog.
type table struct {
	Name    string   `json:"name"`
	Domain  string   `json:"domain"`
	Purpose string   `json:"purpose"`
	Tools   []string `json:"tools,omitempty"`
}

// overview is the wire shape returned by schema://overview. Kept small
// (~3 KB serialized) so an LLM can pull the whole thing into context
// cheaply.
type overview struct {
	Version int      `json:"version"`
	Tables  []table  `json:"tables"`
	Notes   []string `json:"notes"`
}

// Register adds every resource to srv.
func Register(srv *mcp.Server) {
	srv.AddResource(&mcp.Resource{
		Name:        "schema_overview",
		Title:       "Schema overview",
		URI:         SchemaOverviewURI,
		MIMEType:    "application/json",
		Description: "Compact catalog of every Postgres table the indexer fills, with the MCP tools that read each one. Pull this first to ground tool selection.",
	}, schemaOverviewHandler)
}

func schemaOverviewHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	body, err := json.Marshal(schemaOverview())
	if err != nil {
		return nil, err
	}
	uri := SchemaOverviewURI
	if req != nil && req.Params != nil && req.Params.URI != "" {
		uri = req.Params.URI
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(body),
		}},
	}, nil
}

// schemaOverview is the canonical catalog. Keep table names + domains
// aligned with docs/schema/index.md. Tools come from
// internal/mcp/tools/register.go. When either side gains a new entry,
// update this list — there is intentionally no auto-derivation: this
// is the LLM-facing summary and benefits from human-curated phrasing.
func schemaOverview() overview {
	return overview{
		Version: 1,
		Tables: []table{
			// Core ledger
			{Name: "momentums", Domain: "core_ledger", Purpose: "Block headers indexed by height.",
				Tools: []string{"get_momentum_by_height", "get_latest_momentum", "list_momentums", "get_status"}},
			{Name: "account_blocks", Domain: "core_ledger", Purpose: "Every transaction with decoded ABI inputs.",
				Tools: []string{"list_account_blocks", "get_account_block", "list_account_transactions"}},
			{Name: "accounts", Domain: "core_ledger", Purpose: "One row per address; flow metrics, delegation, genesis seed.",
				Tools: []string{"get_account"}},
			{Name: "balances", Domain: "core_ledger", Purpose: "Current balance per (address, token).",
				Tools: []string{"list_account_balances", "list_token_holders"}},
			{Name: "tokens", Domain: "core_ledger", Purpose: "ZTS token registry with current supply + holder/tx counts.",
				Tools: []string{"list_tokens", "get_token"}},
			{Name: "token_mints", Domain: "core_ledger", Purpose: "Every mint event as its own row."},
			{Name: "token_burns", Domain: "core_ledger", Purpose: "Every burn event as its own row."},

			// Pillars + delegation
			{Name: "pillars", Domain: "pillars", Purpose: "Current pillar registry.",
				Tools: []string{"list_pillars", "get_pillar_by_name", "list_pillar_delegators"}},
			{Name: "pillar_updates", Domain: "pillars", Purpose: "Append-only history of pillar config changes."},
			{Name: "delegations", Domain: "pillars", Purpose: "Time-bucketed delegator → pillar intervals."},

			// Sentinels / stakes / plasma
			{Name: "sentinels", Domain: "sentinels_stakes_plasma", Purpose: "Sentinel node registrations.",
				Tools: []string{"list_sentinels"}},
			{Name: "stakes", Domain: "sentinels_stakes_plasma", Purpose: "Staking entries (with ABI-derived cancel_id).",
				Tools: []string{"list_stakes", "list_account_stakes"}},
			{Name: "fusions", Domain: "sentinels_stakes_plasma", Purpose: "Plasma fusion entries (with ABI-derived cancel_id).",
				Tools: []string{"list_fusions", "list_account_fusions"}},

			// Accelerator-Z
			{Name: "projects", Domain: "accelerator_z", Purpose: "Accelerator-Z funding projects.",
				Tools: []string{"list_projects", "get_project"}},
			{Name: "project_phases", Domain: "accelerator_z", Purpose: "Project phases (sub-grants).",
				Tools: []string{"list_project_phases"}},
			{Name: "votes", Domain: "accelerator_z", Purpose: "Pillar votes on projects/phases.",
				Tools: []string{"list_project_votes"}},

			// Rewards
			{Name: "reward_transactions", Domain: "rewards", Purpose: "Per-event reward receipts.",
				Tools: []string{"list_account_rewards"}},
			{Name: "cumulative_rewards", Domain: "rewards", Purpose: "Running total per (address, type, token).",
				Tools: []string{"get_account_cumulative_rewards"}},

			// Bridge
			{Name: "wrap_token_requests", Domain: "bridge", Purpose: "ZTS → external-chain wrap intents.",
				Tools: []string{"list_bridge_wraps", "list_account_bridge_wraps"}},
			{Name: "unwrap_token_requests", Domain: "bridge", Purpose: "External-chain → ZTS unwrap intents.",
				Tools: []string{"list_bridge_unwraps", "list_account_bridge_unwraps"}},
			{Name: "bridge_networks", Domain: "bridge", Purpose: "Configured destination networks (paginated from BridgeApi)."},
			{Name: "bridge_network_tokens", Domain: "bridge", Purpose: "Per-network token pair configuration (fees, min, redeem delay)."},
			{Name: "bridge_admin", Domain: "bridge", Purpose: "Singleton with current administrator + halt state."},
			{Name: "bridge_guardians", Domain: "bridge", Purpose: "Active guardian set."},
			{Name: "bridge_orchestrator_info", Domain: "bridge", Purpose: "Singleton with orchestrator parameters."},
			{Name: "bridge_security_info", Domain: "bridge", Purpose: "Singleton with security delay parameters."},

			// Daily snapshots
			{Name: "network_stat_histories", Domain: "daily_snapshots", Purpose: "Daily network-wide totals + activity."},
			{Name: "token_stat_histories", Domain: "daily_snapshots", Purpose: "Daily per-token mints/burns + carried state."},
			{Name: "pillar_stat_histories", Domain: "daily_snapshots", Purpose: "Daily per-pillar weight + delegator count."},
			{Name: "bridge_stat_histories", Domain: "daily_snapshots", Purpose: "Daily per-(network, chain, token) wrap/unwrap volume."},
		},
		Notes: []string{
			"All amounts/balances/supplies are int64 (BIGINT) — JSON-encoded as strings to dodge JavaScript Number precision loss.",
			"All timestamps are Unix seconds (BIGINT).",
			"All hashes and addresses are lowercase hex/zenon strings.",
			"No foreign keys: joins are by hash/address/standard. The indexer writes per momentum in a transaction.",
		},
	}
}
