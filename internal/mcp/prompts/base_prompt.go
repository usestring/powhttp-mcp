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

### Extract Data from JSON Responses
1. **Search**: Find entries by host/path
2. **Query directly**: ` + "`powhttp_query_body(entry_ids: [...], expression: \".data.items[].name\")`" + `

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

## Tips
- **Check content type first**: Only query JSON responses (` + "`application/json`" + `)
- **Use clusters**: ` + "`powhttp_extract_endpoints`" + ` groups similar requests for batch querying
- **Check ` + "`content_type_hint`" + `** on clusters before calling ` + "`query_body`" + ` or ` + "`validate_schema`" + `
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
