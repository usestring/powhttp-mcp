package tools

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers all tools with the MCP server.
func Register(srv *sdkmcp.Server, d *Deps) {
	// Tool 1: powhttp_sessions_list
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_sessions_list",
		Description: "List all powhttp sessions with their entry counts",
	}, ToolSessionsList(d))

	// Tool 2: powhttp_session_active
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_session_active",
		Description: "Get the currently active powhttp session",
	}, ToolSessionActive(d))

	// Tool 3: powhttp_search_entries
	searchDesc := "Search HTTP entries with filters and free text query. Free-text query searches across URLs, query parameters, and headers"
	if d.Config.IndexBody {
		searchDesc += " and body content"
	}
	searchDesc += " (tokens ANDed). Use header_contains for substring matching on header fields"
	if d.Config.IndexBody {
		searchDesc += ", body_contains for body text substring matching"
	}
	searchDesc += "."
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_search_entries",
		Description: searchDesc,
	}, ToolSearchEntries(d))

	// Tool 4: powhttp_get_entry
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_entry",
		Description: "Get details of a specific HTTP entry. Returns summary, body (compact by default), and available_data metadata. Set include_headers=true for headers; set body_mode to 'schema' for structure-only or 'full' for complete body.",
	}, ToolGetEntry(d))

	// Tool 5: powhttp_get_tls
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_tls",
		Description: "Get TLS handshake events for a connection",
	}, ToolGetTLS(d))

	// Tool 6: powhttp_get_http2_stream
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_get_http2_stream",
		Description: "Get HTTP/2 frame details for a stream",
	}, ToolGetHTTP2Stream(d))

	// Tool 7: powhttp_fingerprint
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_fingerprint",
		Description: "Generate HTTP, TLS, and HTTP/2 fingerprints for anti-bot comparison",
	}, ToolFingerprint(d))

	// Tool 8: powhttp_diff_entries
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_diff_entries",
		Description: "Compare two HTTP entries to find anti-bot detection differences",
	}, ToolDiffEntries(d))

	// Tool 9: powhttp_extract_endpoints
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_extract_endpoints",
		Description: "Group HTTP entries by endpoint pattern into clusters (e.g., /api/users/{id}). Returns clusters with cluster_id, host, method, path_template, count, category (api/page/asset/data/other), and lightweight stats (status_profile, error_rate, avg_resp_bytes, has_auth). Filter with scope.host, scope.method (pre-clustering), or filters.category, filters.min_count (post-clustering) to narrow results. Pass cluster_id to describe_endpoint, infer_schema, or query_body for deeper analysis. For GraphQL APIs (POST /graphql), use powhttp_graphql_operations instead.",
	}, ToolExtractEndpoints(d))

	// Tool 10: powhttp_describe_endpoint
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_describe_endpoint",
		Description: "Generate a detailed description of an endpoint cluster. Returns headers, auth_signals, query_keys, request_body_shape, response_body_shape, and example entries. Requires cluster_id from extract_endpoints. Use this for a quick endpoint overview including body structure; use infer_schema for deeper multi-sample field statistics.",
	}, ToolDescribeEndpoint(d))

	// Tool 11: powhttp_trace_flow
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_trace_flow",
		Description: "Trace related requests (redirects, dependent calls) around a seed entry",
	}, ToolTraceFlow(d))

	// Tool 12: powhttp_validate_schema
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_validate_schema",
		Description: "Validate HTTP entry bodies against a schema (Go struct, Zod, or JSON Schema)",
	}, ToolValidateSchema(d))

	// Tool 13: powhttp_query_body
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_query_body",
		Description: "Extract specific values from HTTP bodies across one or many entries. Returns a values array, per-entry results, and hints. Requires entry_ids (from search_entries) or cluster_id (from extract_endpoints). Expression language is auto-detected from content-type (JQ for JSON/YAML, CSS for HTML, XPath for XML, regex for text, key name for forms); set mode to override. Use get_entry instead for viewing raw body content.",
	}, ToolQueryBody(d))

	// Tool 14: powhttp_infer_schema
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_infer_schema",
		Description: "Infer a merged schema from multiple HTTP entry bodies. Returns a shape result keyed by content_category (json, xml, csv, html, form) with format-specific analysis: JSON/YAML get a JSON Schema plus field_stats (frequency, required/optional, formats, enums); other types get structural outlines. Use this tool for deep multi-sample analysis when describe_endpoint's shape overview is insufficient. Requires entry_ids or cluster_id.",
	}, ToolInferSchema(d))

	// Tool 15: powhttp_graphql_operations
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_graphql_operations",
		Description: "Cluster GraphQL traffic by operation name and type (query/mutation/subscription). Returns operation clusters with counts, error counts, fields, and example entry IDs, plus a traffic summary. Searches all POST entries and validates request bodies, so it works regardless of endpoint path. Filter by operation_type to see only queries, mutations, or subscriptions. Use this INSTEAD OF extract_endpoints when analyzing GraphQL APIs -- extract_endpoints collapses all GraphQL operations into one cluster. Use graphql_inspect to drill into a specific operation, or graphql_errors to find failures.",
	}, ToolGraphQLOperations(d))

	// Tool 16: powhttp_graphql_inspect
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_graphql_inspect",
		Description: "Parse and inspect individual GraphQL operations. Returns operation details with variables_schema, response_schema, and field statistics. Requires entry_ids or operation_name; omitting both returns an error. Use this to understand what an operation sends and receives. Use graphql_errors instead when looking for failures.",
	}, ToolGraphQLInspect(d))

	// Tool 17: powhttp_graphql_errors
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_graphql_errors",
		Description: "Extract and categorize GraphQL errors from responses. Returns error groups with messages, paths, and extensions, plus a summary distinguishing partial failures (data + errors) from full failures (null data + errors). Requires entry_ids or operation_name; omitting both returns an error. Use graphql_inspect instead for operation schema details.",
	}, ToolGraphQLErrors(d))
}
