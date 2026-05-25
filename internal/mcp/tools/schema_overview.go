package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// schemaTable is one row of the catalog. The MCP tools that read a
// table are listed in `Tools` so an LLM can map a table back to the
// tool surface without re-reading the tool catalog.
type schemaTable struct {
	Name    string   `json:"name"`
	Domain  string   `json:"domain"`
	Purpose string   `json:"purpose"`
	Tools   []string `json:"tools,omitempty"`
}

// schemaOverview is the wire shape returned by get_schema_overview.
// Kept small (~3 KB serialized) so an LLM can pull the whole thing
// into context cheaply.
type schemaOverview struct {
	Version int           `json:"version"`
	Tables  []schemaTable `json:"tables"`
	Notes   []string      `json:"notes"`
}

func registerSchemaOverview(srv *mcp.Server, _ *repository.Repositories) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_schema_overview",
		Description: "Compact catalog of every Postgres table the indexer fills, with the " +
			"MCP tools that read each one. Call this first when the user's question doesn't " +
			"obviously map to a single tool — it grounds tool selection in the data model. " +
			"Returns {version, tables: [{name, domain, purpose, tools}], notes: [string]}.",
	}, getSchemaOverview())
}

func getSchemaOverview() func(context.Context, *mcp.CallToolRequest, *struct{}) (*mcp.CallToolResult, any, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, any, error) {
		return jsonResult(catalog())
	}
}

// catalog is the canonical schema overview. Keep table names + domains
// aligned with docs/schema/index.md. Tools come from register.go. When
// either side gains a new entry, update this list — there is
// intentionally no auto-derivation: this is the LLM-facing summary
// and benefits from human-curated phrasing.
func catalog() schemaOverview {
	return schemaOverview{
		Version: 1,
		Tables: []schemaTable{
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
				Tools: []string{"list_pillars", "get_pillar_by_name", "list_pillar_delegators", "get_pillar_voting_history"}},
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
				Tools: []string{"list_projects", "get_project", "get_project_voting_report"}},
			{Name: "project_phases", Domain: "accelerator_z", Purpose: "Project phases (sub-grants).",
				Tools: []string{"list_project_phases"}},
			{Name: "votes", Domain: "accelerator_z", Purpose: "Pillar votes on projects/phases.",
				Tools: []string{"list_project_votes", "get_project_voting_report", "get_pillar_voting_history"}},

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
