package prompts

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleDebugGraphQL implements the GraphQL API debugging workflow.
func HandleDebugGraphQL(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args := req.Params.Arguments

		host := ""
		operation := ""
		if args != nil {
			if v, ok := args["host"]; ok {
				host = v
			}
			if v, ok := args["operation"]; ok {
				operation = v
			}
		}

		var sb strings.Builder

		// 1. Role/Persona
		sb.WriteString("# Debug GraphQL API from Captured Traffic\n\n")
		sb.WriteString("You are a GraphQL API debugging expert. Your goal is to analyze captured GraphQL traffic, ")
		sb.WriteString("identify operations, inspect schemas, and surface errors with actionable context.\n\n")

		// 2. Task Overview
		sb.WriteString("## Task Overview\n\n")
		sb.WriteString("GraphQL APIs funnel all requests through a single POST endpoint (e.g., /graphql), ")
		sb.WriteString("so REST-oriented tools like extract_endpoints are not useful. Instead, use the dedicated GraphQL tools ")
		sb.WriteString("to cluster by operation name, inspect query/variable/response schemas, and find errors.\n\n")

		// 3. Context Usage Guide
		sb.WriteString("## Context Usage Guide\n\n")
		sb.WriteString("- **powhttp_survey_graphql**: Low cost -- returns operation clusters and traffic summary\n")
		sb.WriteString("- **powhttp_inspect_graphql_operation**: Medium cost -- returns schemas, field stats, and errors for a specific operation. Use `sections` to request only what you need.\n")
		sb.WriteString("- **GraphQL resources** (`powhttp://graphql/{session}/{operation}/{aspect}`): On-demand full data for query, response-schema, field-stats, errors\n")
		sb.WriteString("- **powhttp_query_body**: Use for extracting specific values after inspecting schemas\n")
		sb.WriteString("- **powhttp_get_entry**: Use sparingly -- only when you need full raw body content\n\n")

		// 4. Workflow Steps with Decision Criteria
		sb.WriteString("## Workflow Steps\n\n")

		if operation != "" {
			// Skip survey step -- go directly to inspect
			sb.WriteString("1. **Inspect target operation** -- Get schema details and errors in one call\n")
			sb.WriteString("   - Examine variables_schema and response_schema\n")
			sb.WriteString("   - Review field_stats for frequency and type information\n")
			sb.WriteString("   - Check error_groups for partial failures (data + errors) vs full failures (null data)\n\n")
			sb.WriteString("2. **Drill into specifics** -- Use sections to narrow focus\n")
			sb.WriteString("   - Use `sections=[\"errors\"]` if you only need error analysis\n")
			sb.WriteString("   - Use `sections=[\"query\", \"variables\"]` to focus on the request shape\n\n")
			sb.WriteString("3. **Extract specific values** -- Drill into response data\n")
			sb.WriteString("   - Use powhttp_query_body to extract specific fields based on the schema\n\n")
		} else {
			sb.WriteString("1. **Survey operations** -- Discover all GraphQL operations in traffic\n")
			sb.WriteString("   - Review operation_clusters: name, type, count, error_count\n")
			sb.WriteString("   - High error_count operations need attention first\n")
			sb.WriteString("   - High count operations are core API surface\n\n")
			sb.WriteString("2. **Inspect key operations** -- Get schema details and errors\n")
			sb.WriteString("   - One call returns query, variables_schema, response_schema, field_stats, and errors\n")
			sb.WriteString("   - Use `sections` to request only what you need and save context\n")
			sb.WriteString("   - Fetch resources for large queries or schemas that exceed inline thresholds\n\n")
			sb.WriteString("3. **Extract specific values** -- Drill into response data\n")
			sb.WriteString("   - Use powhttp_query_body for targeted extraction\n\n")
		}

		// 5. Suggested Tools
		sb.WriteString("## Suggested Tools\n\n")
		sb.WriteString("```\n")

		if operation != "" {
			// Direct operation inspection
			sb.WriteString("# Step 1: Inspect the target operation (schema + errors)\n")
			fmt.Fprintf(&sb, "powhttp_inspect_graphql_operation(operation_name=%q)\n\n", operation)
			sb.WriteString("# Step 2: Focus on errors only (if needed)\n")
			fmt.Fprintf(&sb, "powhttp_inspect_graphql_operation(operation_name=%q, sections=[\"errors\"])\n\n", operation)
			sb.WriteString("# Step 3: Extract specific values (use entry_ids from inspect results)\n")
			sb.WriteString("powhttp_query_body(entry_ids=[...], expression=\".data\")\n")
		} else {
			// Full workflow
			sb.WriteString("# Step 1: Survey all operations\n")
			if host != "" {
				fmt.Fprintf(&sb, "powhttp_survey_graphql(scope={host: %q})\n\n", host)
			} else {
				sb.WriteString("powhttp_survey_graphql()\n\n")
			}
			sb.WriteString("# Step 2: Inspect a specific operation (schema + errors in one call)\n")
			sb.WriteString("powhttp_inspect_graphql_operation(operation_name=\"<name from step 1>\")\n\n")
			sb.WriteString("# Step 3: Extract specific response values\n")
			sb.WriteString("powhttp_query_body(entry_ids=[...], expression=\".data.users[].name\")\n")
		}
		sb.WriteString("```\n\n")

		// 6. Output Format
		sb.WriteString("## Expected Output Format\n\n")
		sb.WriteString("Present your findings as:\n\n")
		sb.WriteString("1. **Traffic Overview**: Total operations, types (query/mutation/subscription), hosts\n")
		sb.WriteString("2. **Operation Details**: For each key operation -- name, type, fields, schema summary\n")
		sb.WriteString("3. **Error Analysis**: Error types, affected operations, partial vs full failures\n")
		sb.WriteString("4. **Key Findings**: Patterns, anomalies, or issues discovered\n\n")

		// 7. Error Recovery
		sb.WriteString("## If Things Go Wrong\n\n")
		sb.WriteString("- **No GraphQL traffic found?** Check `powhttp_sessions_list` to verify traffic was captured. The endpoint may use a custom path -- set scope.path to override.\n")
		sb.WriteString("- **Operation not found?** Run `powhttp_survey_graphql()` first to list all available operation names.\n")
		sb.WriteString("- **Empty schemas?** The operation may use GET requests or non-JSON bodies. Check `powhttp_search_entries(filters={path_contains: \"graphql\"})`.\n")
		sb.WriteString("- **No errors but unexpected behavior?** Use `powhttp_query_body` to inspect actual response data values.\n\n")

		// 8. Success Criteria
		sb.WriteString("## Success Criteria\n\n")
		sb.WriteString("Task is complete when:\n")
		sb.WriteString("- GraphQL operations are cataloged with their schemas, OR\n")
		sb.WriteString("- Errors are identified with root cause analysis, OR\n")
		sb.WriteString("- Sufficient information is captured to understand the GraphQL API surface\n\n")

		// 9. Constraints
		sb.WriteString("## Constraints\n\n")
		sb.WriteString("- Do NOT use powhttp_extract_endpoints for GraphQL traffic -- it clusters by URL path which is the same for all operations\n")
		sb.WriteString("- Do NOT fetch full entry bodies unless schema inspection is insufficient\n")
		sb.WriteString("- Focus on operations with high error_count first\n")
		sb.WriteString("- STOP after analyzing the most important operations -- edge cases can be investigated later\n")

		return &sdkmcp.GetPromptResult{
			Description: "Guide for debugging GraphQL APIs from captured traffic",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: sb.String()},
				},
			},
		}, nil
	}
}
