package resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSchemaOverview_ContainsKnownTables(t *testing.T) {
	t.Parallel()
	o := schemaOverview()
	if o.Version != 1 {
		t.Errorf("version: want 1, got %d", o.Version)
	}
	if len(o.Tables) < 25 {
		t.Errorf("expected at least 25 tables in overview, got %d", len(o.Tables))
	}
	// Spot-check a handful of representative tables from different
	// domains. Full coverage would just duplicate the literal in
	// schema.go; this confirms the wiring + JSON shape.
	wantTables := []string{"momentums", "accounts", "pillars", "projects", "wrap_token_requests"}
	have := make(map[string]bool, len(o.Tables))
	for _, tb := range o.Tables {
		have[tb.Name] = true
	}
	for _, name := range wantTables {
		if !have[name] {
			t.Errorf("schema overview missing %q", name)
		}
	}
}

func TestSchemaOverviewHandler_ReturnsJSON(t *testing.T) {
	t.Parallel()
	res, err := schemaOverviewHandler(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: SchemaOverviewURI},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(res.Contents))
	}
	c := res.Contents[0]
	if c.URI != SchemaOverviewURI {
		t.Errorf("uri: want %q, got %q", SchemaOverviewURI, c.URI)
	}
	if c.MIMEType != "application/json" {
		t.Errorf("mime: want application/json, got %q", c.MIMEType)
	}
	var decoded overview
	if err := json.Unmarshal([]byte(c.Text), &decoded); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if decoded.Version == 0 || len(decoded.Tables) == 0 {
		t.Errorf("decoded body looks empty: %+v", decoded)
	}
}
