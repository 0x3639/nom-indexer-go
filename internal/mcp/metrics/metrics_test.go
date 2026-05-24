package metrics

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMiddleware_ObservesToolCalls(t *testing.T) {
	t.Parallel()

	m := New()
	mw := m.Middleware()

	// Wrap a fake handler that returns success for "ok_tool" and an
	// error for "err_tool", and records pass-through for non-tool methods.
	var seen []string
	handler := func(_ context.Context, method string, req mcp.Request) (mcp.Result, error) {
		seen = append(seen, method)
		if method != methodCallTool {
			return nil, nil
		}
		r := req.(*mcp.ServerRequest[*mcp.CallToolParamsRaw])
		switch r.Params.Name {
		case "ok_tool":
			return &mcp.CallToolResult{}, nil
		case "err_tool":
			return nil, errors.New("boom")
		case "tool_iserror":
			return &mcp.CallToolResult{IsError: true}, nil
		}
		return &mcp.CallToolResult{}, nil
	}

	wrapped := mw(handler)
	ctx := context.Background()

	// Non-tool method: passes through, not observed.
	if _, err := wrapped(ctx, "initialize", nil); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	for _, name := range []string{"ok_tool", "err_tool", "tool_iserror"} {
		req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{
			Params: &mcp.CallToolParamsRaw{Name: name},
		}
		_, _ = wrapped(ctx, methodCallTool, req)
	}

	if len(seen) != 4 {
		t.Fatalf("expected 4 handler invocations, got %d", len(seen))
	}

	// Scrape /metrics and confirm the counter rows.
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	m.Handler().ServeHTTP(rec, r)
	body, _ := io.ReadAll(rec.Body)
	got := string(body)

	for _, want := range []string{
		`nom_mcp_tool_calls_total{status="ok",tool="ok_tool"} 1`,
		`nom_mcp_tool_calls_total{status="error",tool="err_tool"} 1`,
		`nom_mcp_tool_calls_total{status="error",tool="tool_iserror"} 1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing metric line:\n  want: %s\n  got first 400 chars:\n%s",
				want, firstN(got, 400))
		}
	}
}

func TestToolName_DefendsAgainstBadShape(t *testing.T) {
	t.Parallel()
	if got := toolName(nil); got != "unknown" {
		t.Errorf("nil req: want unknown, got %q", got)
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
