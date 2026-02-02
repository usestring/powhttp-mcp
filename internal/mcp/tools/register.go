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
		Description: "Group HTTP entries by endpoint pattern into clusters (e.g., /api/users/{id}). Returns clusters with cluster_id, host, method, path_template, count, category (api/page/asset/data/other), and lightweight stats (status_profile, error_rate, avg_resp_bytes, has_auth). Filter with scope.host, scope.method (pre-clustering), or filters.category, filters.min_count (post-clustering) to narrow results. Pass cluster_id to describe_endpoint, infer_schema, or query_body for deeper analysis. For GraphQL APIs (POST /graphql), use powhttp_survey_graphql instead.",
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

	// Tool 15: powhttp_survey_graphql
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_survey_graphql",
		Description: "Cluster GraphQL traffic by operation name and type (query/mutation/subscription). Returns operation clusters with counts, error counts, fields, and example entry IDs, plus a traffic summary. Searches all POST entries and validates request bodies, so it works regardless of endpoint path. Filter by operation_type to see only queries, mutations, or subscriptions. Use this INSTEAD OF extract_endpoints when analyzing GraphQL APIs -- extract_endpoints collapses all GraphQL operations into one cluster. Use inspect_graphql_operation to drill into a specific operation's schema and errors.",
	}, ToolSurveyGraphQL(d))

	// Tool 16: powhttp_inspect_graphql_operation
	AddTool(srv, &sdkmcp.Tool{
		Name:        "powhttp_inspect_graphql_operation",
		Description: "Inspect a single GraphQL operation: parse the query, infer variable and response schemas, compute field statistics, and collect errors. Merges schema inspection and error analysis into one call. Use the sections parameter ([\"query\", \"variables\", \"response_shape\", \"errors\", \"fragment_warnings\", \"fragment_coverage\", \"response_variants\"]; default: all) to request only what you need. Use `fragment_warnings` to detect missing union/interface fragments (response objects with only `__typename`). Use `fragment_coverage` for a comprehensive matrix of query fragments vs response types. Use `response_variants` when the same operation returns different response shapes depending on variable values. Returns compact inline summaries plus resource URIs (powhttp://graphql/{session}/{operation}/{aspect}) for full data. Requires entry_ids or operation_name. Use survey_graphql first to discover operation names.",
	}, ToolInspectGraphQLOperation(d))
}
