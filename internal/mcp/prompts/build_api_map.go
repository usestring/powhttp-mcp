package prompts

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleBuildAPIMap implements the API catalog workflow.
func HandleBuildAPIMap(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args := req.Params.Arguments

		host := ""
		processName := ""
		if args != nil {
			if v, ok := args["host"]; ok {
				host = v
			}
			if v, ok := args["process_name"]; ok {
				processName = v
			}
		}

		var sb strings.Builder

		// 1. Role/Persona
		sb.WriteString("# Build API Map from Captured Traffic\n\n")
		sb.WriteString("You are an API reverse engineering expert specializing in building comprehensive endpoint catalogs from HTTP traffic. ")
		sb.WriteString("Your goal is to document API structure, authentication patterns, and data schemas systematically.\n\n")

		// 2. Task Overview
		sb.WriteString("## Task Overview\n\n")
		sb.WriteString("This workflow helps you catalog all API endpoints from captured HTTP traffic, including request/response schemas, ")
		sb.WriteString("authentication requirements, and common usage patterns. Build a complete, actionable API map.\n\n")

		// 3. Context Usage Guide
		sb.WriteString("## Context Usage Guide\n\n")
		sb.WriteString("- **Tools** return summaries and data shapes - use these for most analysis\n")
		sb.WriteString("- **Resources** return full raw data - high context cost, only fetch when you need actual body content\n")
		sb.WriteString("- The describe_endpoint tool provides body schemas without fetching full bodies\n")
		sb.WriteString("- Use selective sampling - you don't need to describe every endpoint\n\n")

		// 4. Workflow Steps with Decision Criteria
		sb.WriteString("## Workflow Steps\n\n")
		sb.WriteString("1. **Extract endpoints** - Cluster similar requests into endpoint groups\n")
		sb.WriteString("   - Start without filters to see overall traffic patterns\n")
		sb.WriteString("   - Filter by host if targeting specific API. Use `*.domain` prefix to include subdomains (e.g., scope.host: `*.example.com`)\n")
		sb.WriteString("   - Filter by process_name if isolating app behavior\n\n")
		sb.WriteString("2. **Review clusters** - Browse the endpoint catalog\n")
		sb.WriteString("   - Filter by category: `filters={category: \"api\"}` to focus on structured API endpoints\n")
		sb.WriteString("   - High-count `api` clusters are core endpoints (prioritize these)\n")
		sb.WriteString("   - Check `stats.error_rate` to spot failing endpoints\n")
		sb.WriteString("   - Check `stats.has_auth` to identify authenticated endpoints early\n\n")
		sb.WriteString("3. **Describe endpoints** - Get detailed info about specific endpoints\n")
		sb.WriteString("   - Focus on authenticated endpoints first\n")
		sb.WriteString("   - Document body schemas for POST/PUT endpoints\n")
		sb.WriteString("   - Note authentication signals (cookies, tokens, API keys)\n\n")

		sb.WriteString("## Suggested Tools\n\n")
		sb.WriteString("```\n")
		sb.WriteString("# Step 1: Extract endpoint clusters\n")

		scopeParts := []string{}
		if host != "" {
			scopeParts = append(scopeParts, fmt.Sprintf("host: \"%s\"", host))
		}
		if processName != "" {
			scopeParts = append(scopeParts, fmt.Sprintf("process_name: \"%s\"", processName))
		}

		if len(scopeParts) > 0 {
			sb.WriteString(fmt.Sprintf("powhttp_extract_endpoints(scope={%s})\n", strings.Join(scopeParts, ", ")))
		} else {
			sb.WriteString("powhttp_extract_endpoints()\n")
		}
		sb.WriteString("\n")

		sb.WriteString("# Step 2: Describe interesting endpoints\n")
		sb.WriteString("powhttp_describe_endpoint(cluster_id=\"<cluster_id>\")\n")
		sb.WriteString("\n")

		sb.WriteString("# Step 3: Get example requests (when schema details needed)\n")
		sb.WriteString("powhttp_get_entry(entry_id=\"<entry_id>\")\n")
		sb.WriteString("\n")
		sb.WriteString("# Step 4: Discover authentication patterns\n")
		sb.WriteString("# 4a. Endpoint-level auth signals (fastest - already computed per cluster)\n")
		sb.WriteString("powhttp_describe_endpoint(cluster_id=\"<cluster_id>\")  # check auth_signals field\n")
		sb.WriteString("# 4b. Search for specific auth headers across all traffic\n")
		sb.WriteString("powhttp_search_entries(filters={header_contains: \"authorization\"})\n")
		sb.WriteString("powhttp_search_entries(filters={header_contains: \"x-api-key\"})\n")
		sb.WriteString("# 4c. Trace auth flow (token propagation, cookie origins)\n")
		sb.WriteString("powhttp_trace_flow(seed_entry_id=\"<authenticated_entry>\")  # check edge_type_summary for auth_chain, session_cookie_origin\n")
		sb.WriteString("```\n\n")

		// 5. Output Format Specification
		sb.WriteString("## Expected Output Format\n\n")
		sb.WriteString("Present your API map as:\n\n")
		sb.WriteString("1. **Overview**: Summary of total endpoints, primary hosts, and traffic volume\n")
		sb.WriteString("2. **Endpoint Catalog**: Table or structured list with:\n")
		sb.WriteString("   - Method + Path template\n")
		sb.WriteString("   - Purpose/description\n")
		sb.WriteString("   - Auth requirements\n")
		sb.WriteString("   - Request/response schemas (for key endpoints)\n")
		sb.WriteString("3. **Key Findings**: Notable patterns, authentication flows, or API quirks\n\n")

		// 6. Constraints
		sb.WriteString("## Constraints\n\n")
		sb.WriteString("- Do NOT fetch full resources unless body schema cannot be inferred from describe_endpoint\n")
		sb.WriteString("- Do NOT document every single endpoint - focus on top 5-7 most important ones\n")
		sb.WriteString("- STOP after cataloging core functionality - edge cases can be added later\n")
		sb.WriteString("- Do NOT include implementation details that aren't visible in HTTP traffic\n\n")

		// 7. Error Recovery
		sb.WriteString("## If Things Go Wrong\n\n")
		sb.WriteString("- **No clusters found?** Check `powhttp_sessions_list` to verify traffic was captured\n")
		sb.WriteString("- **Too many clusters?** Add host or process_name filters to narrow scope\n")
		sb.WriteString("- **Missing body schemas?** Use `powhttp_get_entry` with max_bytes to sample actual bodies\n")
		sb.WriteString("- **Inconsistent schemas?** Check if endpoint handles multiple content types or API versions\n\n")

		// 8. Success Criteria
		sb.WriteString("## Success Criteria\n\n")
		sb.WriteString("Task is complete when:\n")
		sb.WriteString("- Core API endpoints are documented with schemas, OR\n")
		sb.WriteString("- Authentication patterns are identified and documented, OR\n")
		sb.WriteString("- Sufficient information is captured to replicate key API calls\n\n")

		// 9. Testing Guidance
		sb.WriteString("## Testing Your API Map\n\n")
		sb.WriteString(fmt.Sprintf("When validating your API map, test requests at: %s\n", cfg.PowHTTPProxyURL))
		sb.WriteString("This allows you to capture and compare your reconstructed requests against original traffic.\n\n")

		// 10. Cluster Information Details
		sb.WriteString("## Cluster Information\n\n")
		sb.WriteString("Each cluster shows:\n")
		sb.WriteString("- **Host** - The target domain\n")
		sb.WriteString("- **Method** - HTTP method (GET, POST, etc.)\n")
		sb.WriteString("- **Path Template** - Normalized path with IDs replaced (e.g., /users/{id})\n")
		sb.WriteString("- **Count** - Number of requests (indicates importance)\n")
		sb.WriteString("- **Category** - Endpoint type (api, page, asset, data, other) for quick triage\n")
		sb.WriteString("- **Stats** - Error rate, auth signals, and response sizes for prioritization\n")
		sb.WriteString("- **Examples** - Sample entry IDs for inspection\n\n")

		sb.WriteString("## Endpoint Description Details\n\n")
		sb.WriteString("The describe tool provides:\n")
		sb.WriteString("- **Typical Headers** - Common request headers and their frequency\n")
		sb.WriteString("- **Auth Signals** - `cookies_present`, `bearer_present`, `custom_auth_headers` (x-api-key, x-auth-token, x-access-token)\n")
		sb.WriteString("- **Query Keys** - Stable parameters vs volatile/session-specific ones\n")
		sb.WriteString("- **Body Shape** - JSON schema inferred from request/response bodies\n\n")
		sb.WriteString("**IMPORTANT**: Always include body shape analysis for POST/PUT endpoints - the schema reveals data structure requirements.\n\n")

		sb.WriteString("## Auth Discovery Strategy\n\n")
		sb.WriteString("Use a layered approach to map authentication:\n")
		sb.WriteString("1. **`describe_endpoint` -> `auth_signals`**: Fastest - shows cookies, bearer tokens, and custom auth headers per endpoint cluster\n")
		sb.WriteString("2. **`search_entries` -> `header_contains`**: Find specific auth patterns across all traffic (e.g., `\"bearer\"`, `\"x-api-key\"`)\n")
		sb.WriteString("3. **`trace_flow` -> `edge_type_summary`**: Map auth propagation - `auth_chain` (token reuse), `session_cookie_origin` (Set-Cookie -> Cookie), `same_auth` (shared Authorization), `same_api_key` (shared API key)\n\n")

		sb.WriteString("## Tips\n\n")
		sb.WriteString("- Start broad to see overall API surface, then narrow focus\n")
		sb.WriteString("- High-count clusters (>100 requests) are usually core endpoints\n")
		sb.WriteString("- Use `powhttp_trace_flow` to understand request sequences, auth flows, and dependencies\n")
		sb.WriteString("- Check `edge_type_summary` on trace results - `auth_chain` and `session_cookie_origin` edges reveal auth flows\n")
		sb.WriteString("- Look for versioned endpoints (/v1/, /v2/) - document the latest version\n")

		return &sdkmcp.GetPromptResult{
			Description: "Guide for building an API endpoint catalog",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: sb.String()},
				},
			},
		}, nil
	}
}
