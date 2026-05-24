// Package metrics owns the Prometheus registry for the MCP server and
// the middleware that observes per-tool call counts + durations.
//
// Metrics are exported on a separate listener (port 9091 by default)
// alongside the equivalent REST API listener on 9090 — keeps the
// public /mcp endpoint free of operational data and matches the
// API's split-listener convention.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// methodCallTool is the MCP method dispatched for every tool invocation.
// Hard-coded because the SDK does not export the constant (lowercased in
// protocol.go). Keep in sync with the SDK if the JSON-RPC method ever
// changes; the SDK's TestMiddleware uses the same literal.
const methodCallTool = "tools/call"

// Metrics owns the prometheus.Registry and its registered collectors.
// Build one via New(); register Middleware() on the MCP server and hand
// Handler() to the metrics http.Server.
type Metrics struct {
	registry  *prometheus.Registry
	callTotal *prometheus.CounterVec
	callDur   *prometheus.HistogramVec
}

// New constructs a Metrics with the standard process + Go runtime
// collectors plus the MCP tool-call counter and duration histogram.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	callTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nom_mcp",
		Name:      "tool_calls_total",
		Help:      "Total MCP tool invocations, labeled by tool name and status (ok/error).",
	}, []string{"tool", "status"})

	callDur := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nom_mcp",
		Name:      "tool_call_duration_seconds",
		Help:      "Duration of MCP tool invocations, in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"tool", "status"})

	reg.MustRegister(callTotal, callDur)

	return &Metrics{
		registry:  reg,
		callTotal: callTotal,
		callDur:   callDur,
	}
}

// Handler returns the promhttp.HandlerFor for the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Middleware returns an SDK receiving-middleware that observes every
// tools/call invocation. Non-tool methods (initialize, tools/list,
// resources/read, etc.) pass through untouched — they are cheap and
// uniform; per-tool latency is what operators care about.
//
// Status label resolution:
//   - "error" if the handler returned a Go error (transport-level fault)
//     or set CallToolResult.IsError (tool-reported business error).
//   - "ok" otherwise.
//
// Tool name comes from CallToolParamsRaw.Name; if a malformed call
// arrives without a name, we record it as "unknown" to keep the
// histogram bounded.
func (m *Metrics) Middleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != methodCallTool {
				return next(ctx, method, req)
			}
			tool := toolName(req)
			start := time.Now()
			res, err := next(ctx, method, req)
			status := classify(res, err)
			m.callTotal.WithLabelValues(tool, status).Inc()
			m.callDur.WithLabelValues(tool, status).Observe(time.Since(start).Seconds())
			return res, err
		}
	}
}

// toolName extracts the tool name from a tools/call request. The
// receiving side passes a *mcp.ServerRequest[*mcp.CallToolParamsRaw];
// fall back to "unknown" if the SDK ever changes shape so the
// middleware never panics on a bad call.
func toolName(req mcp.Request) string {
	if req == nil {
		return "unknown"
	}
	r, ok := req.(*mcp.ServerRequest[*mcp.CallToolParamsRaw])
	if !ok || r == nil || r.Params == nil || r.Params.Name == "" {
		return "unknown"
	}
	return r.Params.Name
}

func classify(res mcp.Result, err error) string {
	if err != nil {
		return "error"
	}
	if r, ok := res.(*mcp.CallToolResult); ok && r != nil && r.IsError {
		return "error"
	}
	return "ok"
}
