package prompts

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleBasePrompt serves the base prompt optimization guide.
func HandleBasePrompt(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		message := `# Efficient Tool Usage Guide

## Search: Parameter Decision Table

| Goal | Parameter | Example |
|------|-----------|---------|
| Find entries by URL, path, or query content | ` + "`query`" + ` | ` + "`query: \"api users\"`" + ` |
| Find entries with specific header content | ` + "`header_contains`" + ` | ` + "`header_contains: \"bearer\"`" + ` or ` + "`header_contains: \"content-type: json\"`" + ` |
| Find entries with specific body content | ` + "`body_contains`" + ` | ` + "`body_contains: \"error\"`" + ` |
| Check if a header exists (name only) | ` + "`header_name`" + ` | ` + "`header_name: \"authorization\"`" + ` |
| Broad discovery across URL+headers+body | ` + "`query`" + ` | ` + "`query: \"authorization bearer\"`" + ` |

**Key rules**:
- ` + "`query`" + ` searches across URLs, headers, and body tokens (ANDed: all terms must match somewhere)
- ` + "`header_contains`" + ` does substring matching on full header fields (name + value)
- ` + "`body_contains`" + ` does substring matching on decoded body text (cached entries only)
- Check ` + "`matched_in`" + ` on results to see where each match came from (url/header/body)
- Check ` + "`searched_scope`" + ` to know if body search coverage was partial

## Search Results (Token-Optimized)
- **THIN by default**: Returns entry_id, URL, method, status, http_version, and content-type hint only
- Set ` + "`include_details: true`" + ` only if filtering by TLS/HTTP2/process info

## Two-Step Header Inspection
1. Search: ` + "`powhttp_search_entries(filters: {header_contains: \"authorization\"})`" + `
2. Inspect: ` + "`powhttp_get_entry(entry_id: \"...\", include_headers: true, body_mode: \"schema\")`" + `

## Get Entry (Token-Optimized)
- **No headers by default**: Headers excluded to save tokens
- Check ` + "`available_data`" + ` in the response to see what was included and plan follow-up calls
- **Body modes**:
  - ` + "`compact`" + ` (default): Arrays trimmed to 3 items with "... (N more)" indicator
  - ` + "`schema`" + `: JSON schema only (structure without values)
  - ` + "`full`" + `: Complete body (use sparingly)

## Recommended Workflows

### Extract Data from Responses
1. **Search**: Find entries by host/path
2. **Query directly**: ` + "`powhttp_query_body(entry_ids: [...], expression: \".data.items[].name\")`" + `
   - **JSON/YAML**: JQ expressions (auto-detected). E.g., ` + "`.data.items[].name`" + `
   - **HTML**: CSS selectors. E.g., ` + "`h1.title`" + `, ` + "`.product-price`" + `
   - **XML**: XPath. E.g., ` + "`//item/name`" + `
   - **Plain text**: Regex. E.g., ` + "`status:\\s*(\\w+)`" + `
   - **Forms**: Key name. E.g., ` + "`email`" + ` or ` + "`*`" + ` for all
   - Set ` + "`mode`" + ` to override auto-detection

### Discover Authentication Patterns
1. **Cluster-level**: ` + "`powhttp_describe_endpoint(cluster_id)`" + ` - check ` + "`auth_signals`" + ` (cookies, bearer, custom headers)
2. **Search**: ` + "`powhttp_search_entries(filters: {header_contains: \"authorization\"})`" + ` - find auth headers across traffic
3. **Trace flow**: ` + "`powhttp_trace_flow(seed_entry_id)`" + ` - check ` + "`edge_type_summary`" + ` for ` + "`auth_chain`" + `, ` + "`session_cookie_origin`" + `, ` + "`same_auth`" + `, ` + "`same_api_key`" + `

### Debug Anti-Bot Issues
` + "```" + `
powhttp_search_entries(filters: {host: "api.example.com"}, include_details: true)
powhttp_fingerprint(entry_id)
` + "```" + `

## JQ Quick Reference
- ` + "`.data.items[].name`" + ` - Extract from nested arrays
- ` + "`.[] | select(.type == \"product\")`" + ` - Filter array elements
- ` + "`. | keys`" + ` - List all top-level keys

## Understand Data Shape
Three tools for understanding body structure, from quick to deep:
1. **Quick overview**: ` + "`powhttp_describe_endpoint(cluster_id)`" + ` - includes ` + "`request_body_shape`" + ` and ` + "`response_body_shape`" + ` (JSON schema, XML hierarchy, HTML outline, etc.)
2. **Deep analysis**: ` + "`powhttp_infer_schema(entry_ids or cluster_id)`" + ` - multi-sample merged schema with field statistics (frequency, required/optional, format detection, enums). Handles all content types automatically.
3. **Single entry**: ` + "`powhttp_get_entry(entry_id, body_mode: \"schema\")`" + ` - quick look at one entry's structure

**Workflow**: Use ` + "`powhttp_infer_schema`" + ` before ` + "`powhttp_query_body`" + ` to discover which fields exist and their types, then extract specific values with ` + "`powhttp_query_body`" + `.

## GraphQL APIs
GraphQL APIs funnel all requests through a single POST endpoint (e.g., /graphql), so ` + "`powhttp_extract_endpoints`" + ` clusters them as one endpoint. Use the dedicated GraphQL tools instead:

1. **Survey**: ` + "`powhttp_graphql_operations(scope={host: ...})`" + ` - clusters by operation name/type (the GraphQL equivalent of ` + "`powhttp_extract_endpoints`" + `)
2. **Inspect**: ` + "`powhttp_graphql_inspect(operation_name=..., host=...)`" + ` - parses variables_schema, response_schema, and field_stats
3. **Errors**: ` + "`powhttp_graphql_errors(operation_name=..., host=...)`" + ` - finds partial failures (data + errors) vs full failures (null data)

**Auto-detection**: ` + "`powhttp_extract_endpoints`" + ` automatically detects GraphQL endpoints by probing request bodies and emits a hint with the host and request count. Follow that hint directly -- call ` + "`powhttp_graphql_operations`" + ` with the suggested ` + "`scope.host`" + `. ` + "`powhttp_graphql_operations`" + ` validates all POST request bodies regardless of URL path, so it works for custom GraphQL endpoints too -- just call it without a scope to search all POST traffic.

## Tips
- **Any content type**: ` + "`powhttp_query_body`" + ` auto-detects the expression language from the content-type
- **Use clusters**: ` + "`powhttp_extract_endpoints`" + ` groups similar requests for batch querying
- **Check ` + "`content_type_hint`" + `** on clusters to pick the right expression syntax
- **Deduplicate**: Set ` + "`deduplicate: true`" + ` to remove duplicate values
`

		return &sdkmcp.GetPromptResult{
			Description: "Essential guide for efficient tool usage",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: message},
				},
			},
		}, nil
	}
}
