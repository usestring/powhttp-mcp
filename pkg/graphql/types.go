// Package graphql provides lightweight GraphQL request body parsing
// and traffic analysis types. It extracts operation names, types, and
// top-level fields from GraphQL HTTP request bodies without a full AST parser.
package graphql

import (
	"github.com/invopop/jsonschema"

	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

// ParsedOperation represents a single parsed GraphQL operation.
type ParsedOperation struct {
	Name          string   `json:"name"`                     // Operation name ("anonymous" if unnamed)
	Type          string   `json:"type"`                     // query, mutation, or subscription
	Fields        []string `json:"fields,omitempty"`         // Top-level field selections
	RawQuery      string   `json:"raw_query,omitempty"`      // Raw query string (if include_query)
	Variables     any      `json:"variables,omitempty"`      // Variables object (raw)
	HasVariables  bool     `json:"has_variables"`            // Whether variables were present
	BatchIndex    int      `json:"batch_index,omitempty"`    // Index in batched request (0 for non-batched)
	ParseFailed   bool     `json:"parse_failed,omitempty"`   // True if query string could not be parsed
	OperationName string   `json:"operation_name,omitempty"` // Raw operationName from JSON body
}

// ParseResult contains the result of parsing a GraphQL request body.
type ParseResult struct {
	Operations []ParsedOperation `json:"operations"`
	IsBatched  bool              `json:"is_batched"` // True if the body was a JSON array
}

// OperationCluster groups GraphQL entries by operation name and type.
type OperationCluster struct {
	Name         string   `json:"name"`                    // Operation name
	Type         string   `json:"type"`                    // query, mutation, or subscription
	Count        int      `json:"count"`                   // Total requests
	ErrorCount   int      `json:"error_count"`             // Requests with GraphQL errors
	Fields       []string `json:"fields,omitempty"`        // Union of top-level fields across samples
	HasVariables bool     `json:"has_variables"`           // Any sample used variables
	EntryIDs     []string `json:"example_entry_ids"`       // Example entry IDs (up to 5)
}

// TrafficSummary summarizes GraphQL traffic across all entries.
type TrafficSummary struct {
	TotalRequests  int      `json:"total_requests"`
	QueryCount     int      `json:"query_count"`
	MutationCount  int      `json:"mutation_count"`
	SubscriptionCount int  `json:"subscription_count"`
	BatchedCount   int      `json:"batched_count"`    // Entries that contained batched ops
	AnonymousCount int      `json:"anonymous_count"`  // Operations without a name
	UniqueOps      int      `json:"unique_operations"`
	Hosts          []string `json:"hosts,omitzero"`
}

// Error represents a single GraphQL error from a response.
type Error struct {
	Message    string   `json:"message"`
	Path       []any    `json:"path,omitempty"`
	Locations  []any    `json:"locations,omitempty"`
	Extensions any      `json:"extensions,omitempty"`
}

// ErrorGroup groups errors from a single entry.
type ErrorGroup struct {
	EntryID        string  `json:"entry_id"`
	OperationName  string  `json:"operation_name,omitempty"`
	Errors         []Error `json:"errors"`
	IsPartial      bool    `json:"is_partial"`       // data != null && errors present
	IsFullFailure  bool    `json:"is_full_failure"`  // data == null && errors present
}

// ErrorSummary summarizes GraphQL errors across entries.
type ErrorSummary struct {
	EntriesChecked    int `json:"entries_checked"`
	EntriesWithErrors int `json:"entries_with_errors"`
	TotalErrors       int `json:"total_errors"`
	PartialFailures   int `json:"partial_failures"`
	FullFailures      int `json:"full_failures"`
}

// InspectedOperation contains parsed operation details with inferred schemas.
type InspectedOperation struct {
	ParsedOperation
	VariablesSchema *jsonschema.Schema `json:"variables_schema,omitempty"`
	ResponseSchema  *jsonschema.Schema `json:"response_schema,omitempty"`
	FieldStats      []js.FieldStat     `json:"field_stats,omitempty"`
}
