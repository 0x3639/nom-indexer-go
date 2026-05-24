# MCP resources

**v1 advertises no MCP resources.** All data the server offers is
exposed through [tools](tools.md). The schema catalog that earlier
revisions served at `schema://overview` is now the
[`get_schema_overview`](tools.md#status--schema) tool — same payload,
called by the LLM on demand.

## Why no resources?

MCP has two ways to surface data: **tools** (LLM-callable functions)
and **resources** (read-only context blobs the user or LLM can attach).
The two render differently in clients — Claude Desktop, for example,
shows resources as a separate submenu inside the connector picker,
distinct from the per-tool toggles. For a query-only server like
nom-indexer that has nothing the user needs to manually attach, the
extra UI affordance was confusing rather than helpful: it suggested
nom-indexer was set up differently from other connectors when in fact
the data was simply being exposed twice.

Converting the schema catalog to a tool:

- keeps the LLM-on-demand pattern that actually mattered (the LLM
  reaches for it when grounding tool selection),
- removes the manual-attach UI noise,
- folds the schema catalog into the same per-tool metrics + auth path
  the other 30 tools already use.

## Future resources

If a future iteration ships data that genuinely benefits from
user-driven attachment (e.g. a per-table schema fragment to drop into
a conversation as context, or a snapshot file too large to fit in a
single tool call), the resource layer is easy to add back — the SDK
supports it directly via `srv.AddResource(...)`. See
[Known issues](../reference/known-issues.md) for the current deferral
list.
