package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/graphql"
	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

// textResult wraps markdown text in a CallToolResult.
func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: text},
		},
	}
}

// hybridResult returns markdown text as the primary content block
// and the structured data as a JSON secondary block.
func hybridResult(text string, data any) *sdkmcp.CallToolResult {
	content := []sdkmcp.Content{
		&sdkmcp.TextContent{Text: text},
	}
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			content = append(content, &sdkmcp.TextContent{Text: string(b)})
		}
	}
	return &sdkmcp.CallToolResult{Content: content}
}

// ---------------------------------------------------------------------------
// survey_graphql renderer
// ---------------------------------------------------------------------------

func renderOperationsText(clusters []graphql.OperationCluster, summary graphql.TrafficSummary) string {
	var b strings.Builder

	// Summary line
	var typeParts []string
	if summary.QueryCount > 0 {
		typeParts = append(typeParts, fmt.Sprintf("%d queries", summary.QueryCount))
	}
	if summary.MutationCount > 0 {
		typeParts = append(typeParts, fmt.Sprintf("%d mutations", summary.MutationCount))
	}
	if summary.SubscriptionCount > 0 {
		typeParts = append(typeParts, fmt.Sprintf("%d subscriptions", summary.SubscriptionCount))
	}

	fmt.Fprintf(&b, "%d requests, %d unique operations", summary.TotalRequests, summary.UniqueOps)
	if len(typeParts) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(typeParts, ", "))
	}
	if len(summary.Hosts) > 0 {
		fmt.Fprintf(&b, "\nHosts: %s", strings.Join(summary.Hosts, ", "))
	}
	b.WriteString("\n\n")

	// Table
	b.WriteString("| Operation | Type | Calls | Errors | Fields |\n")
	b.WriteString("|-----------|------|------:|-------:|--------|\n")

	hasAnyErrors := false
	for _, c := range clusters {
		fields := strings.Join(c.Fields, ", ")
		if fields == "" {
			fields = "-"
		}
		errCol := fmt.Sprintf("%d", c.ErrorCount)
		if c.ErrorCount > 0 {
			errCol = fmt.Sprintf("**%d**", c.ErrorCount)
			hasAnyErrors = true
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s |\n",
			c.Name, c.Type, c.Count, errCol, fields)
	}

	// Entry IDs for follow-up
	b.WriteString("\nEntry IDs for follow-up:\n")
	for _, c := range clusters {
		fmt.Fprintf(&b, "- %s: `%s`\n", c.Name, strings.Join(c.EntryIDs, "`, `"))
	}

	// Next steps (conditional)
	b.WriteString("\n**Next**: ")
	if hasAnyErrors {
		for _, c := range clusters {
			if c.ErrorCount > 0 {
				fmt.Fprintf(&b, "%s has %d errors — investigate with `inspect_graphql_operation(operation_name=%q, sections=[\"errors\"])`. ",
					c.Name, c.ErrorCount, c.Name)
				break
			}
		}
	}
	if len(clusters) > 0 {
		fmt.Fprintf(&b, "Inspect schema with `inspect_graphql_operation(operation_name=%q)`.", clusters[0].Name)
	}
	b.WriteString("\n")

	return b.String()
}

// ---------------------------------------------------------------------------
// inspect_graphql_operation renderer
// ---------------------------------------------------------------------------

// renderInspectionOpts controls which sections appear in the markdown output.
type renderInspectionOpts struct {
	sections  inspectSections
	resources map[string]string // resource URIs by aspect
}

// Inline thresholds
const (
	queryInlineThreshold    = 500  // chars: inline if < 500, resource URI only if >= 500
	queryTruncateThreshold  = 3000 // chars: truncate at 3000 in markdown
	responseLeafThreshold   = 20   // leaf fields: inline if < 20, resource URI if >= 20
	responseLeafMaxShow     = 40   // max fields shown in markdown
	errorsInlineCap         = 10   // max error groups shown inline
)

func renderInspectionText(
	ops []graphql.InspectedOperation,
	errorGroups []graphql.ErrorGroup,
	errSummary graphql.ErrorSummary,
	entryIDs []string,
	entriesMatched int,
	opts renderInspectionOpts,
) string {
	if len(ops) == 0 && len(errorGroups) == 0 {
		return "No operations found.\n"
	}

	var b strings.Builder

	// Header
	opName, opType := "", ""
	if len(ops) > 0 {
		opName = ops[0].Name
		opType = ops[0].Type
	} else if len(errorGroups) > 0 {
		opName = errorGroups[0].OperationName
	}
	if opName != "" {
		fmt.Fprintf(&b, "## %s", opName)
		if opType != "" {
			fmt.Fprintf(&b, " (%s)", opType)
		}
		if entriesMatched > 1 {
			fmt.Fprintf(&b, " — %d entries", entriesMatched)
		}
		b.WriteString("\n\n")
	}

	// Query section
	if opts.sections.query && len(ops) > 0 {
		renderQuerySection(&b, ops, opts.resources)
	}

	// Variables section
	if opts.sections.variables && len(ops) > 0 {
		renderVariableExamples(&b, ops)
	}

	// Response shape section
	if opts.sections.responseShape && len(ops) > 0 {
		renderResponseShapeSection(&b, ops, opts.resources)
	}

	// Errors section
	if opts.sections.errors {
		renderErrorsSection(&b, errorGroups, errSummary, opts.resources)
	}

	// Entry IDs
	if len(entryIDs) > 0 {
		show := entryIDs
		if len(show) > 5 {
			show = show[:5]
		}
		fmt.Fprintf(&b, "Entry IDs: `%s`", strings.Join(show, "`, `"))
		if len(entryIDs) > 5 {
			fmt.Fprintf(&b, " ...and %d more", len(entryIDs)-5)
		}
		b.WriteString("\n\n")
	}

	// Resource URI summary block
	if len(opts.resources) > 0 {
		b.WriteString("**Resources** (fetch for full data):\n")
		for aspect, uri := range opts.resources {
			fmt.Fprintf(&b, "- %s: `%s`\n", aspect, uri)
		}
		b.WriteString("\n")
	}

	// Next steps
	b.WriteString("**Next**: ")
	if len(entryIDs) > 0 {
		ids := entryIDs
		if len(ids) > 2 {
			ids = ids[:2]
		}
		idsStr := formatEntryIDs(ids)
		fmt.Fprintf(&b, "Extract values with `query_body(entry_ids=%s, expression=\".data\")`.", idsStr)
	}
	b.WriteString("\n")

	return b.String()
}

// renderQuerySection renders the query with inline/resource thresholds.
func renderQuerySection(b *strings.Builder, ops []graphql.InspectedOperation, resources map[string]string) {
	var rawQuery string
	for _, op := range ops {
		if op.RawQuery != "" {
			rawQuery = op.RawQuery
			break
		}
	}
	if rawQuery == "" {
		return
	}

	queryLen := len(rawQuery)

	if queryLen < queryInlineThreshold {
		// Small query: show inline
		b.WriteString("### Query\n```graphql\n")
		b.WriteString(rawQuery)
		if !strings.HasSuffix(rawQuery, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	} else {
		// Large query: show truncated + resource URI
		b.WriteString("### Query")
		if uri, ok := resources["query"]; ok {
			fmt.Fprintf(b, " (full: `%s`)", uri)
		}
		b.WriteString("\n```graphql\n")
		display := rawQuery
		if queryLen > queryTruncateThreshold {
			display = rawQuery[:queryTruncateThreshold]
		}
		b.WriteString(display)
		if !strings.HasSuffix(display, "\n") {
			b.WriteByte('\n')
		}
		if queryLen > queryTruncateThreshold {
			fmt.Fprintf(b, "... (%d chars total)\n", queryLen)
		}
		b.WriteString("```\n\n")
	}
}

// renderResponseShapeSection renders the response shape with inline/resource thresholds.
func renderResponseShapeSection(b *strings.Builder, ops []graphql.InspectedOperation, resources map[string]string) {
	var stats []js.FieldStat
	for _, op := range ops {
		if len(op.FieldStats) > 0 {
			stats = op.FieldStats
			break
		}
	}

	if len(stats) == 0 {
		return
	}

	// Filter to leaf fields
	var leaves []js.FieldStat
	for _, s := range stats {
		if s.Type == "object" || s.Type == "array" {
			continue
		}
		leaves = append(leaves, s)
	}

	if len(leaves) == 0 {
		return
	}

	if len(leaves) >= responseLeafThreshold {
		// Large response: show capped + resource URI
		b.WriteString("### Response shape")
		if uri, ok := resources["response_schema"]; ok {
			fmt.Fprintf(b, " (full: `%s`)", uri)
		}
		b.WriteString("\n```\n")

		maxShow := responseLeafMaxShow
		for i, s := range leaves {
			if i >= maxShow {
				fmt.Fprintf(b, "... and %d more fields\n", len(leaves)-maxShow)
				break
			}
			renderFieldLine(b, s)
		}
		b.WriteString("```\n\n")
	} else {
		// Small response: show inline
		b.WriteString("### Response shape\n```\n")
		for _, s := range leaves {
			renderFieldLine(b, s)
		}
		b.WriteString("```\n\n")
	}
}

// renderErrorsSection renders the errors with inline/resource thresholds.
func renderErrorsSection(b *strings.Builder, groups []graphql.ErrorGroup, summary graphql.ErrorSummary, resources map[string]string) {
	fmt.Fprintf(b, "### Errors\nChecked %d entries", summary.EntriesChecked)
	if summary.EntriesWithErrors == 0 {
		b.WriteString(" — no GraphQL errors found.\n\n")
		return
	}

	fmt.Fprintf(b, " | %d with errors | %d total errors",
		summary.EntriesWithErrors, summary.TotalErrors)

	var failParts []string
	if summary.FullFailures > 0 {
		failParts = append(failParts, fmt.Sprintf("%d full failures", summary.FullFailures))
	}
	if summary.PartialFailures > 0 {
		failParts = append(failParts, fmt.Sprintf("%d partial", summary.PartialFailures))
	}
	if len(failParts) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(failParts, ", "))
	}
	b.WriteString("\n\n")

	// Show up to errorsInlineCap groups
	shown := 0
	for _, g := range groups {
		if len(g.Errors) == 0 {
			continue
		}
		if shown >= errorsInlineCap {
			remaining := len(groups) - shown
			if remaining > 0 {
				if uri, ok := resources["errors"]; ok {
					fmt.Fprintf(b, "... and %d more error groups (full: `%s`)\n\n", remaining, uri)
				} else {
					fmt.Fprintf(b, "... and %d more error groups\n\n", remaining)
				}
			}
			break
		}

		fmt.Fprintf(b, "**%s**", g.EntryID)
		if g.OperationName != "" {
			fmt.Fprintf(b, " — %s", g.OperationName)
		}
		if g.IsFullFailure {
			b.WriteString(" — FULL FAILURE")
		} else if g.IsPartial {
			b.WriteString(" — PARTIAL")
		}
		b.WriteByte('\n')

		for i, e := range g.Errors {
			fmt.Fprintf(b, "  %d. %q", i+1, e.Message)
			if len(e.Path) > 0 {
				fmt.Fprintf(b, " at `%s`", formatGQLPath(e.Path))
			}
			if e.Extensions != nil {
				if extJSON, err := json.Marshal(e.Extensions); err == nil && string(extJSON) != "null" {
					fmt.Fprintf(b, " [extensions: %s]", string(extJSON))
				}
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
		shown++
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func renderVariableExamples(b *strings.Builder, ops []graphql.InspectedOperation) {
	// Collect up to 2 distinct variable examples
	var examples []string
	seen := make(map[string]bool)
	for _, op := range ops {
		if !op.HasVariables || op.Variables == nil {
			continue
		}
		raw, err := json.Marshal(op.Variables)
		if err != nil {
			continue
		}
		key := string(raw)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Pretty-print if small, compact if large
		var pretty json.RawMessage
		if err := json.Unmarshal(raw, &pretty); err == nil {
			indented, err := json.MarshalIndent(pretty, "", "  ")
			if err == nil && len(indented) < 500 {
				examples = append(examples, string(indented))
			} else {
				examples = append(examples, string(raw))
			}
		}
		if len(examples) >= 2 {
			break
		}
	}

	if len(examples) == 0 {
		return
	}

	b.WriteString("### Variables")
	if len(examples) > 1 {
		b.WriteString(" (2 examples)")
	}
	b.WriteString("\n")
	for _, ex := range examples {
		b.WriteString("```json\n")
		b.WriteString(ex)
		if !strings.HasSuffix(ex, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	b.WriteByte('\n')
}

func renderFieldLine(b *strings.Builder, s js.FieldStat) {
	// path: type
	fmt.Fprintf(b, "%s: %s", s.Path, s.Type)

	// Annotations
	var notes []string
	if s.Nullable {
		notes = append(notes, "nullable")
	}
	if s.Format != "" {
		notes = append(notes, s.Format)
	}
	if len(s.EnumValues) > 0 {
		notes = append(notes, "enum: "+strings.Join(s.EnumValues, "|"))
	}
	if len(notes) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(notes, ", "))
	}

	// Examples
	exStr := formatExamples(s.Examples, 2)
	if exStr != "" {
		fmt.Fprintf(b, " — %s", exStr)
	}

	b.WriteByte('\n')
}

func formatExamples(examples []any, limit int) string {
	if len(examples) == 0 {
		return ""
	}
	var parts []string
	for _, ex := range examples {
		if len(parts) >= limit {
			break
		}
		switch v := ex.(type) {
		case string:
			s := v
			if len(s) > 40 {
				s = s[:37] + "..."
			}
			parts = append(parts, fmt.Sprintf("%q", s))
		case float64:
			if v == float64(int(v)) {
				parts = append(parts, fmt.Sprintf("%d", int(v)))
			} else {
				parts = append(parts, fmt.Sprintf("%g", v))
			}
		case bool:
			parts = append(parts, fmt.Sprintf("%t", v))
		case nil:
			parts = append(parts, "null")
		default:
			// Skip complex types (objects, arrays)
			continue
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func formatGQLPath(path []any) string {
	var parts []string
	for _, p := range path {
		switch v := p.(type) {
		case string:
			parts = append(parts, v)
		case float64:
			parts = append(parts, fmt.Sprintf("[%d]", int(v)))
		default:
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(parts, ".")
}
