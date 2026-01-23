package tools

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers all tools with the MCP server.
func Register(srv *sdkmcp.Server, d *Deps) {
	// Tool 1: powhttp_sessions_list
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_sessions_list",
		Description: "List all powhttp sessions with their entry counts",
	}, ToolSessionsList(d))

	// Tool 2: powhttp_session_active
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_session_active",
		Description: "Get the currently active powhttp session",
	}, ToolSessionActive(d))

	// Tool 3: powhttp_search_entries
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_search_entries",
		Description: "Search HTTP entries with filters and free text query",
	}, ToolSearchEntries(d))

	// Tool 4: powhttp_get_entry
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_entry",
		Description: "Get full details of a specific HTTP entry",
	}, ToolGetEntry(d))

	// Tool 5: powhttp_get_tls
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_tls",
		Description: "Get TLS handshake events for a connection",
	}, ToolGetTLS(d))

	// Tool 6: powhttp_get_http2_stream
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_http2_stream",
		Description: "Get HTTP/2 frame details for a stream",
	}, ToolGetHTTP2Stream(d))

	// Tool 7: powhttp_fingerprint
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_fingerprint",
		Description: "Generate HTTP, TLS, and HTTP/2 fingerprints for anti-bot comparison",
	}, ToolFingerprint(d))

	// Tool 8: powhttp_diff_entries
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_diff_entries",
		Description: "Compare two HTTP entries to find anti-bot detection differences",
	}, ToolDiffEntries(d))

	// Tool 9: powhttp_extract_endpoints
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_extract_endpoints",
		Description: "Group HTTP entries by endpoint pattern returning clusters (e.g., /api/users/:id)",
	}, ToolExtractEndpoints(d))

	// Tool 10: powhttp_describe_endpoint
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_describe_endpoint",
		Description: "Generate detailed description of an endpoint cluster",
	}, ToolDescribeEndpoint(d))

	// Tool 11: powhttp_trace_flow
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_trace_flow",
		Description: "Trace related requests (redirects, dependent calls) around a seed entry",
	}, ToolTraceFlow(d))

	// Tool 12: powhttp_validate_schema
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_validate_schema",
		Description: "Validate HTTP entry bodies against a schema (Go struct, Zod, or JSON Schema)",
	}, ToolValidateSchema(d))

	// Tool 13: powhttp_query_body
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_query_body",
		Description: "Extract specific fields from request/response bodies using JQ expressions",
	}, ToolQueryBody(d))
}
