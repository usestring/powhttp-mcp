package prompts

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleCompareBrowserProgram implements the compare workflow.
func HandleCompareBrowserProgram(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args := req.Params.Arguments

		baselineHint := ""
		candidateHint := ""
		if args != nil {
			if v, ok := args["baseline_hint"]; ok {
				baselineHint = v
			}
			if v, ok := args["candidate_hint"]; ok {
				candidateHint = v
			}
		}

		var sb strings.Builder

		// 1. Role/Persona
		sb.WriteString("# Compare Browser vs Program Request\n\n")
		sb.WriteString("You are an HTTP traffic analysis expert specializing in anti-bot detection evasion. ")
		sb.WriteString("Your goal is to identify why a program's HTTP request is being detected as a bot by comparing it to legitimate browser traffic.\n\n")

		// 2. Task Overview
		sb.WriteString("## Task Overview\n\n")
		sb.WriteString("This workflow helps you systematically identify detection signals that distinguish automated requests from real browser traffic. ")
		sb.WriteString("Focus on actionable differences that can be corrected in the program's HTTP client.\n\n")

		// 3. Context Usage Guide
		sb.WriteString("## Context Usage Guide\n\n")
		sb.WriteString("- **Tools** return summaries and data shapes - use these for most analysis\n")
		sb.WriteString("- **Resources** return full raw data - high context cost, only fetch when you need actual body content or raw frames\n")
		sb.WriteString("- The tools below provide all the structure you need without fetching expensive resources\n\n")

		// 4. Workflow Steps with Decision Criteria
		sb.WriteString("## Workflow Steps\n\n")
		sb.WriteString("1. **Find baseline request** - Search for the browser request you want to replicate\n")
		sb.WriteString("   - Use specific process names like \"Chrome\", \"Firefox\", or \"Safari\"\n")
		sb.WriteString("   - Verify it's a successful request (status 2xx)\n\n")
		sb.WriteString("2. **Find candidate request** - Search for the program request to compare\n")
		sb.WriteString("   - Match the same endpoint as the baseline\n")
		sb.WriteString("   - Look for blocked/failed requests if testing detection\n\n")
		sb.WriteString("3. **Generate diff** - Compare the two to find detection points\n")
		sb.WriteString("   - Focus on differences marked as high/critical priority\n")
		sb.WriteString("   - If >50 differences, use ignore_headers to filter noise\n\n")

		sb.WriteString("## Suggested Tools\n\n")
		sb.WriteString("```\n")
		sb.WriteString("# Step 1: Search for baseline (browser) request\n")
		if baselineHint != "" {
			sb.WriteString(fmt.Sprintf("powhttp_search_entries(query=\"%s\", filters={process_name: \"Chrome\"})\n", baselineHint))
		} else {
			sb.WriteString("powhttp_search_entries(filters={process_name: \"Chrome\"})\n")
		}
		sb.WriteString("\n")

		sb.WriteString("# Step 2: Search for candidate (program) request\n")
		if candidateHint != "" {
			sb.WriteString(fmt.Sprintf("powhttp_search_entries(query=\"%s\", filters={process_name: \"python\"})\n", candidateHint))
		} else {
			sb.WriteString("powhttp_search_entries(filters={process_name: \"python\"})\n")
		}
		sb.WriteString("\n")

		sb.WriteString("# Step 3: Compare entries\n")
		sb.WriteString("powhttp_diff_entries(baseline_entry_id=\"<browser_entry_id>\", candidate_entry_id=\"<program_entry_id>\")\n")
		sb.WriteString("```\n\n")

		// 5. Output Format Specification
		sb.WriteString("## Expected Output Format\n\n")
		sb.WriteString("Present your findings as:\n\n")
		sb.WriteString("1. **Summary**: 2-3 sentence overview of the key detection issue\n")
		sb.WriteString("2. **Critical Differences**: Bullet list of top 5-7 differences, highest priority first\n")
		sb.WriteString("3. **Recommended Fixes**: Actionable code snippets or library recommendations\n\n")

		// 6. Constraints
		sb.WriteString("## Constraints\n\n")
		sb.WriteString("- Do NOT fetch full resources unless body content is essential for analysis\n")
		sb.WriteString("- Do NOT compare more than 2 entries at a time\n")
		sb.WriteString("- STOP after identifying top 5-7 actionable differences\n")
		sb.WriteString("- Do NOT list every header difference - prioritize detection signals\n\n")

		// 7. Error Recovery
		sb.WriteString("## If Things Go Wrong\n\n")
		sb.WriteString("- **No matching entries?** Check `powhttp_sessions_list` to verify active session\n")
		sb.WriteString("- **Empty diff?** Ensure baseline and candidate hit the same endpoint and method\n")
		sb.WriteString("- **Too many differences?** Use `ignore_headers` option to filter volatile headers (Date, Cookie, etc.)\n")
		sb.WriteString("- **Can't find browser traffic?** Check process_name filter - try \"Chrome\", \"firefox\", \"Safari\"\n\n")

		// 8. Success Criteria
		sb.WriteString("## Success Criteria\n\n")
		sb.WriteString("Task is complete when:\n")
		sb.WriteString("- Root cause of detection is identified (e.g., TLS fingerprint mismatch), OR\n")
		sb.WriteString("- Clear evidence that requests are equivalent (no significant differences found)\n\n")

		// 9. Testing Guidance
		sb.WriteString("## Testing Your Changes\n\n")
		sb.WriteString(fmt.Sprintf("When testing detection fixes, capture traffic at: %s\n", cfg.PowHTTPProxyURL))
		sb.WriteString("This allows side-by-side comparison of before/after traffic in the same session.\n\n")

		// 10. Detection Vectors (Priority Order)
		sb.WriteString("## Detection Vectors (Priority Order)\n\n")
		sb.WriteString("1. **[CRITICAL]** TLS Fingerprint (JA3/JA4) - Blocks 80%+ of bots\n")
		sb.WriteString("2. **[HIGH]** HTTP/2 pseudo-header order - Common in modern sites\n")
		sb.WriteString("3. **[HIGH]** HTTP/2 SETTINGS frame order - Often overlooked\n")
		sb.WriteString("4. **[MEDIUM]** Header presence/order - Detectable pattern\n")
		sb.WriteString("5. **[LOW]** Header value differences - Usually less critical\n\n")

		sb.WriteString("## Tips\n\n")
		sb.WriteString("- Use `powhttp_fingerprint` to inspect a single entry's full fingerprint\n")
		sb.WriteString("- TLS differences often require switching HTTP libraries (e.g., curl_cffi, tls-client)\n")
		sb.WriteString("- HTTP/2 frame order issues may need low-level library configuration\n")

		return &sdkmcp.GetPromptResult{
			Description: "Guide for comparing browser and program requests",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: sb.String()},
				},
			},
		}, nil
	}
}
