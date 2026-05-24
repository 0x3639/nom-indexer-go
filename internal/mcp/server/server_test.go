package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	mcpserver "github.com/0x3639/nom-indexer-go/internal/mcp/server"
)

// TestMCP_InitializeAndToolsList exercises the bare end-to-end
// happy path: bearer auth passes, initialize returns serverInfo,
// tools/list surfaces get_status, and tools/call returns a
// dto.Status. This is the canary for the whole M1 wiring stack.
//
// It uses the official SDK's client against a real httptest server —
// no mocking of MCP protocol framing.
func TestMCP_InitializeAndToolsList(t *testing.T) {
	t.Helper()
	signer, err := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	// Build a minimal server with the SDK directly so we can register
	// a stand-in get_status that doesn't need a DB. The wire shape
	// is what we're proving here — auth + initialize + tools/list +
	// tools/call — not the production handler's DB hit (covered by
	// the per-tool unit tests in internal/mcp/tools).
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "nom-indexer-mcp-test",
		Version: "0.0.0",
	}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_status",
		Description: "stand-in",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, *dto.Status, error) {
		st := &dto.Status{LatestHeight: 42, LatestTimestamp: 1700000000, IndexerLagSeconds: 7, Version: "test"}
		body, _ := json.Marshal(st)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, st, nil
	})

	handler := mcpserver.Auth(signer)(mcpserver.HTTPHandler(srv))
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	tok, err := signer.Issue("mcp-test", time.Hour, []string{"read"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-test-client", Version: "0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: bearerHTTPClient(tok),
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	// tools/list — must surface get_status
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tl := range toolsResult.Tools {
		if tl.Name == "get_status" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tools/list missing get_status; got %v", toolNames(toolsResult.Tools))
	}

	// tools/call get_status — must return a structured Status
	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_status"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if callResult.IsError {
		t.Errorf("tool result IsError = true")
	}
	if callResult.StructuredContent == nil {
		t.Error("StructuredContent nil; want dto.Status")
	}
}

func TestMCP_RejectsMissingAuth(t *testing.T) {
	signer, _ := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	srv := mcp.NewServer(&mcp.Implementation{Name: "x", Version: "0"}, nil)
	handler := mcpserver.Auth(signer)(mcpserver.HTTPHandler(srv))
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, httpSrv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	errObj, _ := body["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("missing error object in 401 body")
	}
	data, _ := errObj["data"].(map[string]any)
	if data["code"] != "missing_token" {
		t.Errorf("error.data.code = %v, want missing_token", data["code"])
	}
}

func TestMCP_AcceptsTokenQueryFallback(t *testing.T) {
	signer, _ := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	tok, _ := signer.Issue("browser", time.Hour, []string{"read"})

	srv := mcp.NewServer(&mcp.Implementation{Name: "x", Version: "0"}, nil)
	handler := mcpserver.Auth(signer)(mcpserver.HTTPHandler(srv))
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()

	// We're not initializing the MCP session here — just proving the
	// auth middleware reads ?token= when the Authorization header is
	// absent. A real client would still pass through after this.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, httpSrv.URL+"?token="+tok, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = nil
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	// 401 means auth failed; anything else (200, 400, 405) means
	// auth at least let the request through.
	if resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("?token= was rejected; status=%d", resp.StatusCode)
	}
}

// bearerHTTPClient returns an http.Client that stamps every request
// with the bearer header. The SDK doesn't have a first-class hook for
// auth headers, but it accepts a custom *http.Client.
func bearerHTTPClient(tok string) *http.Client {
	return &http.Client{
		Transport: bearerRoundTripper{
			base:  http.DefaultTransport,
			token: tok,
		},
	}
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	rc := r.Clone(r.Context())
	rc.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(rc)
}

func toolNames(ts []*mcp.Tool) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

// helper used by other tests later to satisfy the pgx.ErrNoRows path
// without an actual DB connection.
var _ = pgx.ErrNoRows
