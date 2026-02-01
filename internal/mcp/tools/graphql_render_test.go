package tools

import (
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/pkg/graphql"
	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

func TestRenderOperationsText_Basic(t *testing.T) {
	clusters := []graphql.OperationCluster{
		{Name: "GetUser", Type: "query", Count: 15, ErrorCount: 2, Fields: []string{"user"}, EntryIDs: []string{"e1", "e2"}},
		{Name: "CreateOrder", Type: "mutation", Count: 3, ErrorCount: 0, Fields: []string{"createOrder"}, EntryIDs: []string{"e3"}},
	}
	summary := graphql.TrafficSummary{
		TotalRequests: 18,
		QueryCount:    15,
		MutationCount: 3,
		UniqueOps:     2,
		Hosts:         []string{"api.example.com"},
	}

	out := renderOperationsText(clusters, summary)

	// Summary line
	assert.Contains(t, out, "18 requests, 2 unique operations")
	assert.Contains(t, out, "15 queries")
	assert.Contains(t, out, "3 mutations")
	assert.Contains(t, out, "api.example.com")

	// Table header
	assert.Contains(t, out, "| Operation | Type | Calls | Errors | Fields |")

	// Rows
	assert.Contains(t, out, "| GetUser | query | 15 |")
	assert.Contains(t, out, "**2**") // bold error count
	assert.Contains(t, out, "| CreateOrder | mutation | 3 |")

	// Entry IDs
	assert.Contains(t, out, "GetUser: `e1`, `e2`")
	assert.Contains(t, out, "CreateOrder: `e3`")

	// Next steps — should reference new tool names
	assert.Contains(t, out, "GetUser has 2 errors")
	assert.Contains(t, out, `inspect_graphql_operation(operation_name="GetUser", sections=["errors"])`)
	assert.Contains(t, out, `inspect_graphql_operation(operation_name="GetUser")`)
}

func TestRenderOperationsText_NoErrors(t *testing.T) {
	clusters := []graphql.OperationCluster{
		{Name: "ListItems", Type: "query", Count: 5, ErrorCount: 0, Fields: []string{"items"}, EntryIDs: []string{"e1"}},
	}
	summary := graphql.TrafficSummary{
		TotalRequests: 5,
		QueryCount:    5,
		UniqueOps:     1,
	}

	out := renderOperationsText(clusters, summary)

	// Should NOT suggest errors investigation when no errors
	assert.NotContains(t, out, "errors —")
	// Should suggest inspect
	assert.Contains(t, out, `inspect_graphql_operation(operation_name="ListItems")`)
}

func TestRenderInspectionText_AllSections(t *testing.T) {
	ops := []graphql.InspectedOperation{
		{
			ParsedOperation: graphql.ParsedOperation{
				Name:         "GetUser",
				Type:         "query",
				Fields:       []string{"user"},
				RawQuery:     "query GetUser($id: ID!) {\n  user(id: $id) { id name }\n}",
				Variables:    map[string]any{"id": "abc123"},
				HasVariables: true,
			},
			FieldStats: []js.FieldStat{
				{Path: "data", Type: "object"},
				{Path: "data.user", Type: "object"},
				{Path: "data.user.id", Type: "string", Frequency: 1.0, Required: true, Examples: []any{"u_123"}},
				{Path: "data.user.name", Type: "string", Frequency: 1.0, Required: true, Examples: []any{"Alice"}},
				{Path: "data.user.email", Type: "string", Frequency: 0.73, Nullable: true, Examples: []any{"alice@ex.com"}},
			},
		},
	}
	errGroups := []graphql.ErrorGroup{
		{
			EntryID:       "e1",
			OperationName: "GetUser",
			Errors:        []graphql.Error{{Message: "not found", Path: []any{"user"}}},
			IsFullFailure: true,
		},
	}
	errSummary := graphql.ErrorSummary{
		EntriesChecked:    2,
		EntriesWithErrors: 1,
		TotalErrors:       1,
		FullFailures:      1,
	}
	resources := map[string]string{
		"query":           "powhttp://graphql/active/GetUser/query",
		"response_schema": "powhttp://graphql/active/GetUser/response-schema",
		"field_stats":     "powhttp://graphql/active/GetUser/field-stats",
		"errors":          "powhttp://graphql/active/GetUser/errors",
	}
	opts := renderInspectionOpts{
		sections:  allSections(),
		resources: resources,
	}

	out := renderInspectionText(ops, errGroups, errSummary, []string{"e1", "e2"}, 2, opts)

	// Header
	assert.Contains(t, out, "## GetUser (query)")
	assert.Contains(t, out, "2 entries")

	// Query code fence (small query, inline)
	assert.Contains(t, out, "```graphql")
	assert.Contains(t, out, "query GetUser($id: ID!)")

	// Variables
	assert.Contains(t, out, "### Variables")
	assert.Contains(t, out, `"id": "abc123"`)

	// Response shape — should skip object containers
	assert.Contains(t, out, "### Response shape")
	assert.Contains(t, out, "data.user.id: string")
	assert.Contains(t, out, `"u_123"`)
	assert.Contains(t, out, "data.user.email: string (nullable)")
	assert.NotContains(t, out, "data: object")
	assert.NotContains(t, out, "data.user: object")

	// Errors section
	assert.Contains(t, out, "### Errors")
	assert.Contains(t, out, "1 with errors")
	assert.Contains(t, out, "FULL FAILURE")

	// Entry IDs
	assert.Contains(t, out, "Entry IDs: `e1`, `e2`")

	// Resource summary
	assert.Contains(t, out, "**Resources**")

	// Next steps
	assert.Contains(t, out, "query_body")
}

func TestRenderInspectionText_QueryOnly(t *testing.T) {
	ops := []graphql.InspectedOperation{
		{
			ParsedOperation: graphql.ParsedOperation{
				Name:     "GetUser",
				Type:     "query",
				RawQuery: "query GetUser { user { id } }",
			},
			FieldStats: []js.FieldStat{
				{Path: "data.user.id", Type: "string"},
			},
		},
	}
	opts := renderInspectionOpts{
		sections: inspectSections{query: true},
	}

	out := renderInspectionText(ops, nil, graphql.ErrorSummary{}, []string{"e1"}, 1, opts)

	assert.Contains(t, out, "### Query")
	assert.Contains(t, out, "query GetUser")
	// Variables and response shape should NOT appear
	assert.NotContains(t, out, "### Variables")
	assert.NotContains(t, out, "### Response shape")
	assert.NotContains(t, out, "### Errors")
}

func TestRenderInspectionText_ErrorsOnly(t *testing.T) {
	errGroups := []graphql.ErrorGroup{
		{
			EntryID:       "e1",
			OperationName: "GetUser",
			Errors:        []graphql.Error{{Message: "forbidden"}},
			IsPartial:     true,
		},
	}
	errSummary := graphql.ErrorSummary{
		EntriesChecked:    5,
		EntriesWithErrors: 1,
		TotalErrors:       1,
		PartialFailures:   1,
	}
	opts := renderInspectionOpts{
		sections: inspectSections{errors: true},
	}

	out := renderInspectionText(nil, errGroups, errSummary, []string{"e1"}, 1, opts)

	assert.Contains(t, out, "### Errors")
	assert.Contains(t, out, "1 with errors")
	assert.Contains(t, out, "PARTIAL")
	// No query/variables/response sections
	assert.NotContains(t, out, "### Query")
	assert.NotContains(t, out, "### Variables")
	assert.NotContains(t, out, "### Response shape")
}

func TestRenderInspectionText_NoOps(t *testing.T) {
	opts := renderInspectionOpts{sections: allSections()}
	out := renderInspectionText(nil, nil, graphql.ErrorSummary{}, nil, 0, opts)
	assert.Contains(t, out, "No operations found")
}

func TestRenderInspectionText_QueryTruncation(t *testing.T) {
	// Create a query longer than queryInlineThreshold (500 chars)
	longQuery := "query GetUser($id: ID!) {\n" + strings.Repeat("  field\n", 100) + "}"
	require.Greater(t, len(longQuery), queryInlineThreshold)

	ops := []graphql.InspectedOperation{
		{
			ParsedOperation: graphql.ParsedOperation{
				Name:     "GetUser",
				Type:     "query",
				RawQuery: longQuery,
			},
		},
	}
	resources := map[string]string{
		"query": "powhttp://graphql/active/GetUser/query",
	}
	opts := renderInspectionOpts{
		sections:  inspectSections{query: true},
		resources: resources,
	}

	out := renderInspectionText(ops, nil, graphql.ErrorSummary{}, nil, 1, opts)

	// Should show resource URI reference
	assert.Contains(t, out, "powhttp://graphql/active/GetUser/query")
	assert.Contains(t, out, "(full:")
}

func TestRenderInspectionText_ErrorsCapped(t *testing.T) {
	// Create more than errorsInlineCap (10) error groups
	var groups []graphql.ErrorGroup
	for i := 0; i < 15; i++ {
		groups = append(groups, graphql.ErrorGroup{
			EntryID:       strings.Repeat("e", 1) + string(rune('a'+i)),
			OperationName: "GetUser",
			Errors:        []graphql.Error{{Message: "some error"}},
			IsPartial:     true,
		})
	}
	summary := graphql.ErrorSummary{
		EntriesChecked:    20,
		EntriesWithErrors: 15,
		TotalErrors:       15,
		PartialFailures:   15,
	}
	resources := map[string]string{
		"errors": "powhttp://graphql/active/GetUser/errors",
	}
	opts := renderInspectionOpts{
		sections:  inspectSections{errors: true},
		resources: resources,
	}

	out := renderInspectionText(nil, groups, summary, nil, 0, opts)

	// Should show capped message with resource URI
	assert.Contains(t, out, "more error groups")
	assert.Contains(t, out, "powhttp://graphql/active/GetUser/errors")
}

func TestFormatGQLPath(t *testing.T) {
	assert.Equal(t, "user", formatGQLPath([]any{"user"}))
	assert.Equal(t, "user.email", formatGQLPath([]any{"user", "email"}))
	assert.Equal(t, "users.[2].name", formatGQLPath([]any{"users", float64(2), "name"}))
}

func TestFormatExamples(t *testing.T) {
	assert.Equal(t, `"hello"`, formatExamples([]any{"hello"}, 3))
	assert.Equal(t, "42", formatExamples([]any{float64(42)}, 3))
	assert.Equal(t, "3.14", formatExamples([]any{float64(3.14)}, 3))
	assert.Equal(t, "true", formatExamples([]any{true}, 3))
	assert.Equal(t, "null", formatExamples([]any{nil}, 3))
	assert.Equal(t, `"a", "b"`, formatExamples([]any{"a", "b", "c"}, 2))
	assert.Equal(t, "", formatExamples(nil, 3))
	assert.Equal(t, "", formatExamples([]any{map[string]any{"complex": true}}, 3))

	// Long strings get truncated
	long := strings.Repeat("x", 50)
	result := formatExamples([]any{long}, 3)
	require.Contains(t, result, "...")
	assert.Less(t, len(result), 50)
}

func TestTextResult(t *testing.T) {
	result := textResult("hello world")
	require.Len(t, result.Content, 1)
}

func TestHybridResult(t *testing.T) {
	data := map[string]any{"key": "value"}
	result := hybridResult("# Markdown", data)
	require.Len(t, result.Content, 2)

	// First block is markdown text
	tc0, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "# Markdown", tc0.Text)

	// Second block is JSON
	tc1, ok := result.Content[1].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc1.Text, `"key":"value"`)
}

func TestHybridResult_NilData(t *testing.T) {
	result := hybridResult("text only", nil)
	require.Len(t, result.Content, 1)
}
