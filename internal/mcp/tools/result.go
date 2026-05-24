package tools

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// jsonResult builds an MCP tool result from a value v. The wire frame
// carries TWO copies of the data:
//
//   - Content[0] = TextContent with pretty-printed JSON for the LLM
//     to read (and quote back to the user verbatim if useful).
//   - The third return value (structured content) is the same v
//     unchanged for programmatic clients that want strong types
//     without re-parsing the text payload.
//
// Both come from the same v so they can't disagree. The third return is
// typed `any` on purpose: the SDK only auto-derives an outputSchema when
// the generic Out is non-empty, and the schema it generates for
// dto.Page.Data (interface{}) is "data": true, which Claude Desktop's
// validator rejects — causing the entire tool list to silently drop.
// Returning `any` keeps StructuredContent populated while suppressing
// outputSchema generation. The wire shape is documented per tool in
// docs/mcp/tools.md instead.
//
// If v fails to marshal we return an error result rather than failing
// the tool call hard — gives the LLM something to surface to the user.
func jsonResult[T any](v T) (*mcp.CallToolResult, any, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("marshal failed: %v", err)},
			},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, v, nil
}
