package prompts

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleBasePrompt serves the base prompt optimization guide.
func HandleBasePrompt(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		message := `# Efficient Tool Usage Guide

## Search Results (Token-Optimized)
- **THIN by default**: Returns entry_id, URL, method, status, http_version, and content-type hint only.
- **No headers/cookies**: Search never returns headers. Use ` + "`get_entry`" + ` for headers.
- **Content hints**: ` + "`sizes.resp_content_type`" + ` helps identify JSON responses without fetching.
- Set ` + "`include_details: true`" + ` only if filtering by TLS/HTTP2/process info.

## Get Entry (Token-Optimized)
- **No headers by default**: Headers excluded to save tokens.
- **Add headers**: Set ` + "`include_headers: true`" + ` when you need request/response headers.
- **Body modes**:
  - ` + "`schema`" + ` (default): JSON schema only
  - ` + "`preview`" + `: First 2KB of actual data
  - ` + "`full`" + `: Complete body

## Recommended Workflows

### Extract Data from JSON Responses
1. **Search**: Find entries by host/path
2. **Query directly**: ` + "`powhttp_query_body(entry_ids: [...], expression: \".data.items[].name\")`" + `
   - Extracts specific fields in a single call
   - No need to fetch full bodies

### Inspect Headers (Auth, Cookies, etc.)
` + "```" + `
powhttp_get_entry(entry_id, include_headers: true, body_mode: "schema")
` + "```" + `

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
