package prompts

import (
	"context"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleBasePrompt serves the base prompt optimization guide.
// Body-search rows and rules are included only when body indexing is enabled.
func HandleBasePrompt(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		var sb strings.Builder

		sb.WriteString("# Efficient Tool Usage Guide\n\n")

		// --- Search: Parameter Decision Table ---
		sb.WriteString("## Search: Parameter Decision Table\n\n")
		sb.WriteString("| Goal | Parameter | Example |\n")
		sb.WriteString("|------|-----------|--------|\n")
		sb.WriteString("| Find entries by URL, path, or query content | `query` | `query: \"api users\"` |\n")
		sb.WriteString("| Find entries with specific header content | `header_contains` | `header_contains: \"bearer\"` or `header_contains: \"content-type: json\"` |\n")
		if cfg.BodyIndexEnabled {
			sb.WriteString("| Find entries with specific body content | `body_contains` | `body_contains: \"error\"` |\n")
		}
		sb.WriteString("| Check if a header exists (name only) | `header_name` | `header_name: \"authorization\"` |\n")
		if cfg.BodyIndexEnabled {
			sb.WriteString("| Broad discovery across URL+headers+body | `query` | `query: \"authorization bearer\"` |\n")
		} else {
			sb.WriteString("| Broad discovery across URL+headers | `query` | `query: \"authorization bearer\"` |\n")
		}

		sb.WriteString("\n**Key rules**:\n")
		if cfg.BodyIndexEnabled {
			sb.WriteString("- `query` searches across URLs, headers, and body tokens (ANDed: all terms must match somewhere)\n")
		} else {
			sb.WriteString("- `query` searches across URLs and headers (ANDed: all terms must match somewhere)\n")
		}
		sb.WriteString("- `header_contains` does substring matching on full header fields (name + value)\n")
		if cfg.BodyIndexEnabled {
			sb.WriteString("- `body_contains` does substring matching on decoded body text (cached entries only)\n")
			sb.WriteString("- Check `matched_in` on results to see where each match came from (url/header/body)\n")
			sb.WriteString("- Check `searched_scope` to know if body search coverage was partial\n")
		} else {
			sb.WriteString("- Check `matched_in` on results to see where each match came from (url/header)\n")
		}

		// --- Search Results ---
		sb.WriteString("\n## Search Results (Token-Optimized)\n")
		sb.WriteString("- **THIN by default**: Returns entry_id, URL, method, status, http_version, and content-type hint only\n")
		sb.WriteString("- Set `include_details: true` only if filtering by TLS/HTTP2/process info\n")

		// --- Two-Step Header Inspection ---
		sb.WriteString("\n## Two-Step Header Inspection\n")
		sb.WriteString("1. Search: `powhttp_search_entries(filters: {header_contains: \"authorization\"})`\n")
		sb.WriteString("2. Inspect: `powhttp_get_entry(entry_id: \"...\", include_headers: true, body_mode: \"schema\")`\n")

		// --- Get Entry ---
		sb.WriteString("\n## Get Entry (Token-Optimized)\n")
		sb.WriteString("- **No headers by default**: Headers excluded to save tokens\n")
		sb.WriteString("- Check `available_data` in the response to see what was included and plan follow-up calls\n")
		sb.WriteString("- **Body modes**:\n")
		sb.WriteString("  - `compact` (default): Arrays trimmed to 3 items with \"... (N more)\" indicator\n")
		sb.WriteString("  - `schema`: JSON schema only (structure without values)\n")
		sb.WriteString("  - `full`: Complete body (use sparingly)\n")

		// --- Endpoint Discovery ---
		sb.WriteString("\n## Endpoint Discovery\n\n")
		sb.WriteString("After extracting endpoints with `powhttp_extract_endpoints`:\n")
		sb.WriteString("- Each cluster has a `category` (api, page, asset, data, other) and lightweight `stats` (status_profile, error_rate, avg_resp_bytes, has_auth)\n")
		sb.WriteString("- Use `filters.category` to focus: `powhttp_extract_endpoints(filters={category: \"api\"})` shows only API endpoints\n")
		sb.WriteString("- Use `filters.min_count` to filter noise: `filters={min_count: 5}` hides one-off requests\n")
		sb.WriteString("- Use `scope.method` for pre-clustering filter: `scope={method: \"POST\"}` for write operations\n\n")
		sb.WriteString("**Quick triage from stats**:\n")
		sb.WriteString("- `error_rate > 0.1` -> Investigate failures with `powhttp_get_entry`\n")
		sb.WriteString("- `has_auth: true` -> Check auth patterns with `powhttp_describe_endpoint`\n")
		sb.WriteString("- `category: \"api\"` + high count -> Core endpoints, prioritize for `infer_schema`\n")

		// --- Recommended Workflows ---
		sb.WriteString("\n## Recommended Workflows\n")

		sb.WriteString("\n### Extract Data from Responses\n")
		sb.WriteString("1. **Search**: Find entries by host/path\n")
		sb.WriteString("2. **Query directly**: `powhttp_query_body(entry_ids: [...], expression: \".data.items[].name\")`\n")
		sb.WriteString("   - **JSON/YAML**: JQ expressions (auto-detected). E.g., `.data.items[].name`\n")
		sb.WriteString("   - **HTML**: CSS selectors. E.g., `h1.title`, `.product-price`\n")
		sb.WriteString("   - **XML**: XPath. E.g., `//item/name`\n")
		sb.WriteString("   - **Plain text**: Regex. E.g., `status:\\s*(\\w+)`\n")
		sb.WriteString("   - **Forms**: Key name. E.g., `email` or `*` for all\n")
		sb.WriteString("   - Set `mode` to override auto-detection\n")

		sb.WriteString("\n### Discover Authentication Patterns\n")
		sb.WriteString("1. **Cluster-level**: `powhttp_describe_endpoint(cluster_id)` - check `auth_signals` (cookies, bearer, custom headers)\n")
		sb.WriteString("2. **Search**: `powhttp_search_entries(filters: {header_contains: \"authorization\"})` - find auth headers across traffic\n")
		sb.WriteString("3. **Trace flow**: `powhttp_trace_flow(seed_entry_id)` - check `edge_type_summary` for `auth_chain`, `session_cookie_origin`, `same_auth`, `same_api_key`\n")

		sb.WriteString("\n### Debug Anti-Bot Issues\n")
		sb.WriteString("```\n")
		sb.WriteString("powhttp_search_entries(filters: {host: \"api.example.com\"}, include_details: true)\n")
		sb.WriteString("powhttp_fingerprint(entry_id)\n")
		sb.WriteString("```\n")

		// --- JQ Quick Reference ---
		sb.WriteString("\n## JQ Quick Reference\n")
		sb.WriteString("- `.data.items[].name` - Extract from nested arrays\n")
		sb.WriteString("- `.[] | select(.type == \"product\")` - Filter array elements\n")
		sb.WriteString("- `. | keys` - List all top-level keys\n")

		// --- Understand Data Shape ---
		sb.WriteString("\n## Understand Data Shape\n")
		sb.WriteString("Three tools for understanding body structure, from quick to deep:\n")
		sb.WriteString("1. **Quick overview**: `powhttp_describe_endpoint(cluster_id)` - includes `request_body_shape` and `response_body_shape` (JSON schema, XML hierarchy, HTML outline, etc.)\n")
		sb.WriteString("2. **Deep analysis**: `powhttp_infer_schema(entry_ids or cluster_id)` - multi-sample merged schema with field statistics (frequency, required/optional, format detection, enums). Handles all content types automatically.\n")
		sb.WriteString("3. **Single entry**: `powhttp_get_entry(entry_id, body_mode: \"schema\")` - quick look at one entry's structure\n")
		sb.WriteString("\n**Workflow**: Use `powhttp_infer_schema` before `powhttp_query_body` to discover which fields exist and their types, then extract specific values with `powhttp_query_body`.\n")

		// --- GraphQL APIs ---
		sb.WriteString("\n## GraphQL APIs\n")
		sb.WriteString("GraphQL APIs funnel all requests through a single POST endpoint (e.g., /graphql), so `powhttp_extract_endpoints` clusters them as one endpoint. Use the dedicated GraphQL tools instead:\n\n")
		sb.WriteString("1. **Survey**: `powhttp_graphql_operations(scope={host: ...})` - clusters by operation name/type (the GraphQL equivalent of `powhttp_extract_endpoints`)\n")
		sb.WriteString("2. **Inspect**: `powhttp_graphql_inspect(operation_name=..., host=...)` - parses variables_schema, response_schema, and field_stats\n")
		sb.WriteString("3. **Errors**: `powhttp_graphql_errors(operation_name=..., host=...)` - finds partial failures (data + errors) vs full failures (null data)\n")
		sb.WriteString("\n**Auto-detection**: `powhttp_extract_endpoints` automatically detects GraphQL endpoints by probing request bodies and emits a hint with the host and request count. Follow that hint directly -- call `powhttp_graphql_operations` with the suggested `scope.host`. `powhttp_graphql_operations` validates all POST request bodies regardless of URL path, so it works for custom GraphQL endpoints too -- just call it without a scope to search all POST traffic.\n")

		// --- Tips ---
		sb.WriteString("\n## Tips\n")
		sb.WriteString("- **Filter by category**: `filters.category=\"api\"` skips static assets and pages, focuses on structured API endpoints\n")
		sb.WriteString("- **Check stats first**: `error_rate` and `has_auth` on clusters help prioritize which endpoints to drill into\n")
		sb.WriteString("- **Any content type**: `powhttp_query_body` auto-detects the expression language from the content-type\n")
		sb.WriteString("- **Check `content_type_hint`** on clusters to pick the right expression syntax\n")
		sb.WriteString("- **Deduplicate**: Set `deduplicate: true` to remove duplicate values\n")

		return &sdkmcp.GetPromptResult{
			Description: "Essential guide for efficient tool usage",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: sb.String()},
				},
			},
		}, nil
	}
}
