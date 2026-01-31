package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/graphql"
	"github.com/usestring/powhttp-mcp/pkg/jsonschema"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ---------------------------------------------------------------------------
// Input / Output types
// ---------------------------------------------------------------------------

// GraphQLOperationsInput is the input for powhttp_graphql_operations.
type GraphQLOperationsInput struct {
	SessionID     string                    `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Scope         *GraphQLOperationsScope   `json:"scope,omitempty" jsonschema:"Filtering scope"`
	OperationType string                    `json:"operation_type,omitempty" jsonschema:"Filter by operation type. Valid values: query, mutation, subscription"`
	Limit         int                       `json:"limit,omitempty" jsonschema:"Max clusters to return (default: 50)"`
	Offset        int                       `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

// GraphQLOperationsScope defines the filtering scope for GraphQL operations.
type GraphQLOperationsScope struct {
	Host         string `json:"host,omitempty" jsonschema:"Filter by host"`
	Path         string `json:"path,omitempty" jsonschema:"Filter by URL path substring. By default, searches all POST requests and validates bodies. Set this to narrow the search to a specific path."`
	ProcessName  string `json:"process_name,omitempty" jsonschema:"Filter by process name"`
	PID          int    `json:"pid,omitempty" jsonschema:"Filter by process ID"`
	TimeWindowMs int64  `json:"time_window_ms,omitempty" jsonschema:"Relative time window (ms)"`
	SinceMs      int64  `json:"since_ms,omitempty" jsonschema:"Unix timestamp (ms) lower bound"`
	UntilMs      int64  `json:"until_ms,omitempty" jsonschema:"Unix timestamp (ms) upper bound"`
}

// GraphQLOperationsOutput is the output for powhttp_graphql_operations.
type GraphQLOperationsOutput struct {
	Clusters []graphql.OperationCluster `json:"operation_clusters"`
	Summary  graphql.TrafficSummary     `json:"traffic_summary"`
	Hint     string                     `json:"hint,omitempty"`
}

// GraphQLInspectInput is the input for powhttp_graphql_inspect.
type GraphQLInspectInput struct {
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs      []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to inspect. When both entry_ids and operation_name are provided, only the named operation within the given entries is inspected. Either entry_ids or operation_name is required."`
	OperationName string   `json:"operation_name,omitempty" jsonschema:"GraphQL operation name to find and inspect. Either entry_ids or operation_name is required."`
	Host          string   `json:"host,omitempty" jsonschema:"Filter search by host (used with operation_name; ignored when entry_ids is provided)"`
	IncludeQuery  *bool    `json:"include_query,omitempty" jsonschema:"Include raw query string in output (default: true)"`
	MaxEntries    int      `json:"max_entries,omitempty" jsonschema:"Max entries to inspect (default: 20, max: 100)"`
}

// GraphQLInspectOutput is the output for powhttp_graphql_inspect.
type GraphQLInspectOutput struct {
	Operations      []json.RawMessage `json:"operations"`
	EntriesChecked  int               `json:"entries_checked"`
	EntriesMatched  int               `json:"entries_matched"`
	Hint            string            `json:"hint,omitempty"`
}

// GraphQLErrorsInput is the input for powhttp_graphql_errors.
type GraphQLErrorsInput struct {
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs      []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to check. When both entry_ids and operation_name are provided, only the named operation within the given entries is checked. Either entry_ids or operation_name is required."`
	OperationName string   `json:"operation_name,omitempty" jsonschema:"GraphQL operation name to find and check. Either entry_ids or operation_name is required."`
	Host          string   `json:"host,omitempty" jsonschema:"Filter search by host (used with operation_name; ignored when entry_ids is provided)"`
	ErrorsOnly    *bool    `json:"errors_only,omitempty" jsonschema:"Return only entries with GraphQL errors (default: true). Set to false to include all entries."`
	MaxEntries    int      `json:"max_entries,omitempty" jsonschema:"Max entries to check (default: 20, max: 100)"`
}

// GraphQLErrorsOutput is the output for powhttp_graphql_errors.
type GraphQLErrorsOutput struct {
	ErrorGroups []graphql.ErrorGroup   `json:"error_groups"`
	Summary     graphql.ErrorSummary   `json:"summary"`
	Hint        string                 `json:"hint,omitempty"`
}

// ---------------------------------------------------------------------------
// Tool: powhttp_graphql_operations
// ---------------------------------------------------------------------------

// ToolGraphQLOperations clusters GraphQL traffic by operation name and type.
func ToolGraphQLOperations(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLOperationsInput) (*sdkmcp.CallToolResult, GraphQLOperationsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLOperationsInput) (*sdkmcp.CallToolResult, GraphQLOperationsOutput, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		if input.OperationType != "" {
			switch input.OperationType {
			case "query", "mutation", "subscription":
			default:
				return nil, GraphQLOperationsOutput{}, ErrInvalidInput("operation_type must be 'query', 'mutation', or 'subscription'")
			}
		}

		limit := input.Limit
		if limit <= 0 {
			limit = 50
		}

		// Search all POST requests and let parseGraphQLEntry filter by body.
		searchFilters := &types.SearchFilters{
			Method: "POST",
		}

		if input.Scope != nil {
			if input.Scope.Host != "" {
				searchFilters.Host = input.Scope.Host
			}
			if input.Scope.Path != "" {
				searchFilters.PathContains = input.Scope.Path
			}
			if input.Scope.ProcessName != "" {
				searchFilters.ProcessName = input.Scope.ProcessName
			}
			if input.Scope.PID != 0 {
				searchFilters.PID = input.Scope.PID
			}
			if input.Scope.TimeWindowMs != 0 {
				searchFilters.TimeWindowMs = input.Scope.TimeWindowMs
			}
			if input.Scope.SinceMs != 0 {
				searchFilters.SinceMs = input.Scope.SinceMs
			}
			if input.Scope.UntilMs != 0 {
				searchFilters.UntilMs = input.Scope.UntilMs
			}
		}

		searchResp, err := d.Search.Search(ctx, &types.SearchRequest{
			SessionID: sessionID,
			Filters:   searchFilters,
			Limit:     graphqlSearchLimit,
		})
		if err != nil {
			return nil, GraphQLOperationsOutput{}, WrapPowHTTPError(err)
		}

		if len(searchResp.Results) == 0 {
			return nil, GraphQLOperationsOutput{
				Hint: "No POST requests found. Try powhttp_extract_endpoints() to see what endpoints exist, or powhttp_search_entries(filters={method: \"POST\"}) to find POST traffic.",
			}, nil
		}

		// Parse GraphQL bodies from discovered entries using the shared cache.
		type parsedEntry struct {
			entryID string
			result  *graphql.ParseResult
			host    string
			hasErr  bool // response has GraphQL errors
		}

		var parsed []parsedEntry
		hosts := make(map[string]bool)

		for _, sr := range searchResp.Results {
			entryID := sr.Summary.EntryID

			pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID)
			if !ok {
				continue
			}

			// Check for GraphQL errors in response
			hasErr := false
			entry, err := d.FetchEntry(ctx, sessionID, entryID) // already cached
			if err == nil {
				respBody, respCT, respErr := d.DecodeBody(entry, "response")
				if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
					hasErr = responseHasGraphQLErrors(respBody)
				}

				if u, err := url.Parse(entry.URL); err == nil {
					hosts[u.Host] = true
				}
			}

			parsed = append(parsed, parsedEntry{
				entryID: entryID,
				result:  pr,
				host:    sr.Summary.URL,
				hasErr:  hasErr,
			})
		}

		if len(parsed) == 0 {
			return nil, GraphQLOperationsOutput{
				Hint: "Found POST requests but none contained valid GraphQL request bodies (JSON with a \"query\" field). Use powhttp_get_entry(entry_id=..., body_mode=\"preview\") to inspect the raw body format, or powhttp_extract_endpoints() to see endpoint patterns.",
			}, nil
		}

		// Cluster operations by name + type
		type clusterKey struct {
			Name string
			Type string
		}
		clusterMap := make(map[clusterKey]*graphql.OperationCluster)
		summary := graphql.TrafficSummary{}

		for _, pe := range parsed {
			if pe.result.IsBatched {
				summary.BatchedCount++
			}
			for _, op := range pe.result.Operations {
				summary.TotalRequests++

				switch op.Type {
				case "query":
					summary.QueryCount++
				case "mutation":
					summary.MutationCount++
				case "subscription":
					summary.SubscriptionCount++
				}

				if op.Name == "anonymous" {
					summary.AnonymousCount++
				}

				key := clusterKey{Name: op.Name, Type: op.Type}

				if input.OperationType != "" && op.Type != input.OperationType {
					continue
				}

				cluster, ok := clusterMap[key]
				if !ok {
					cluster = &graphql.OperationCluster{
						Name:   op.Name,
						Type:   op.Type,
						Fields: op.Fields,
					}
					clusterMap[key] = cluster
				}

				cluster.Count++
				if pe.hasErr {
					cluster.ErrorCount++
				}
				if op.HasVariables {
					cluster.HasVariables = true
				}

				// Merge fields
				fieldSet := make(map[string]bool, len(cluster.Fields))
				for _, f := range cluster.Fields {
					fieldSet[f] = true
				}
				for _, f := range op.Fields {
					if !fieldSet[f] {
						cluster.Fields = append(cluster.Fields, f)
						fieldSet[f] = true
					}
				}

				// Add entry ID as example (up to 5)
				if len(cluster.EntryIDs) < 5 {
					cluster.EntryIDs = append(cluster.EntryIDs, pe.entryID)
				}
			}
		}

		// Collect unique hosts
		hostList := make([]string, 0, len(hosts))
		for h := range hosts {
			hostList = append(hostList, h)
		}
		sort.Strings(hostList)
		summary.Hosts = hostList
		summary.UniqueOps = len(clusterMap)

		// Convert to sorted slice
		clusters := make([]graphql.OperationCluster, 0, len(clusterMap))
		for _, c := range clusterMap {
			clusters = append(clusters, *c)
		}
		sort.Slice(clusters, func(i, j int) bool {
			if clusters[i].Count != clusters[j].Count {
				return clusters[i].Count > clusters[j].Count
			}
			return clusters[i].Name < clusters[j].Name
		})

		// Apply pagination
		total := len(clusters)
		start := input.Offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		clusters = clusters[start:end]

		// Build hint with concrete operation names
		var hint string
		if len(clusters) == 0 && input.OperationType != "" {
			hint = fmt.Sprintf("No %s operations found. Remove operation_type filter to see all operations.", input.OperationType)
		} else if len(clusters) > 0 {
			parts := []string{}
			if summary.QueryCount > 0 {
				parts = append(parts, fmt.Sprintf("%d queries", summary.QueryCount))
			}
			if summary.MutationCount > 0 {
				parts = append(parts, fmt.Sprintf("%d mutations", summary.MutationCount))
			}
			if summary.SubscriptionCount > 0 {
				parts = append(parts, fmt.Sprintf("%d subscriptions", summary.SubscriptionCount))
			}

			// Prefer an operation with errors for the hint (more actionable for debugging)
			hintOp := clusters[0].Name
			for _, c := range clusters {
				if c.ErrorCount > 0 {
					hintOp = c.Name
					break
				}
			}
			hint = fmt.Sprintf("Found %d operations (%s). Use powhttp_graphql_inspect(operation_name=%q) for schema details, or powhttp_graphql_errors() to find failures.",
				summary.UniqueOps, strings.Join(parts, ", "), hintOp)
		}

		return nil, GraphQLOperationsOutput{
			Clusters: clusters,
			Summary:  summary,
			Hint:     hint,
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Tool: powhttp_graphql_inspect
// ---------------------------------------------------------------------------

// ToolGraphQLInspect parses and inspects individual GraphQL operations.
func ToolGraphQLInspect(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLInspectInput) (*sdkmcp.CallToolResult, GraphQLInspectOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLInspectInput) (*sdkmcp.CallToolResult, GraphQLInspectOutput, error) {
		if len(input.EntryIDs) == 0 && input.OperationName == "" {
			return nil, GraphQLInspectOutput{}, ErrInvalidInput("either entry_ids or operation_name is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		includeQuery := true
		if input.IncludeQuery != nil {
			includeQuery = *input.IncludeQuery
		}

		maxEntries := input.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 20
		}
		if maxEntries > 100 {
			maxEntries = 100
		}

		// Resolve entry IDs
		entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, input.EntryIDs, input.OperationName, input.Host, maxEntries)
		if err != nil {
			return nil, GraphQLInspectOutput{}, err
		}

		if len(entryIDs) == 0 {
			hint := fmt.Sprintf("No entries found for operation %q. Run powhttp_graphql_operations() to see all operation names.", input.OperationName)
			return nil, GraphQLInspectOutput{Hint: hint}, nil
		}

		// Inspect each entry
		var operations []json.RawMessage
		entriesChecked := 0
		entriesMatched := 0

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				continue
			}
			entriesChecked++

			// Use the shared parse cache to avoid re-decoding and re-parsing
			// the request body (resolveGraphQLEntryIDs already populated it).
			pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID)
			if !ok {
				continue
			}

			for _, op := range pr.Operations {
				// Filter by operation name if specified
				if input.OperationName != "" && op.Name != input.OperationName {
					continue
				}

				entriesMatched++

				inspected := graphql.InspectedOperation{
					ParsedOperation: op,
				}

				if !includeQuery {
					inspected.RawQuery = ""
				}

				// Infer variables schema
				if op.HasVariables && op.Variables != nil {
					varBytes, err := json.Marshal(op.Variables)
					if err == nil {
						inferred, err := jsonschema.Infer(varBytes)
						if err == nil && inferred != nil {
							inspected.VariablesSchema = inferred.Schema
						}
					}
				}

				// Infer response schema
				respBody, respCT, respErr := d.DecodeBody(entry, "response")
				if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
					inferred, err := jsonschema.Infer(respBody)
					if err == nil && inferred != nil {
						inspected.ResponseSchema = inferred.Schema
						inspected.FieldStats = jsonschema.ComputeFieldStats(inferred.Schema, [][]byte{respBody})
					}
				}

				// Marshal to json.RawMessage to avoid exposing recursive
				// jsonschema.Schema types in the MCP tool output schema.
				opJSON, err := json.Marshal(inspected)
				if err != nil {
					return nil, GraphQLInspectOutput{}, fmt.Errorf("marshaling inspected operation: %w", err)
				}
				operations = append(operations, opJSON)
			}
		}

		// Build hint
		var hint string
		if len(operations) > 0 {
			sampleIDs := entryIDs
			if len(sampleIDs) > 3 {
				sampleIDs = sampleIDs[:3]
			}
			idsStr := formatEntryIDs(sampleIDs)
			hint = fmt.Sprintf("Use powhttp_query_body(entry_ids=%s, expression=\".data\") to extract specific response values, or powhttp_graphql_errors(entry_ids=%s) to check for errors.", idsStr, idsStr)
		}

		return nil, GraphQLInspectOutput{
			Operations:     operations,
			EntriesChecked: entriesChecked,
			EntriesMatched: entriesMatched,
			Hint:           hint,
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Tool: powhttp_graphql_errors
// ---------------------------------------------------------------------------

// ToolGraphQLErrors extracts and categorizes GraphQL errors from responses.
func ToolGraphQLErrors(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLErrorsInput) (*sdkmcp.CallToolResult, GraphQLErrorsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GraphQLErrorsInput) (*sdkmcp.CallToolResult, GraphQLErrorsOutput, error) {
		if len(input.EntryIDs) == 0 && input.OperationName == "" {
			return nil, GraphQLErrorsOutput{}, ErrInvalidInput("either entry_ids or operation_name is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		errorsOnly := true
		if input.ErrorsOnly != nil {
			errorsOnly = *input.ErrorsOnly
		}

		maxEntries := input.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 20
		}
		if maxEntries > 100 {
			maxEntries = 100
		}

		// Resolve entry IDs
		entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, input.EntryIDs, input.OperationName, input.Host, maxEntries)
		if err != nil {
			return nil, GraphQLErrorsOutput{}, err
		}

		if len(entryIDs) == 0 {
			hint := fmt.Sprintf("No entries found for operation %q. Run powhttp_graphql_operations() to see all operation names.", input.OperationName)
			return nil, GraphQLErrorsOutput{Hint: hint}, nil
		}

		// Check each entry for GraphQL errors
		var errorGroups []graphql.ErrorGroup
		summary := graphql.ErrorSummary{}

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				continue
			}
			summary.EntriesChecked++

			// Get operation name from request using the shared parse cache
			// to avoid re-decoding and re-parsing the request body.
			opName := ""
			if pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID); ok && len(pr.Operations) > 0 {
				opName = pr.Operations[0].Name
			}

			// Filter by operation name if specified
			if input.OperationName != "" && opName != input.OperationName {
				continue
			}

			// Parse response for errors
			respBody, respCT, respErr := d.DecodeBody(entry, "response")
			if respErr != nil || respBody == nil || !contenttype.IsJSON(respCT) {
				continue
			}

			var respData struct {
				Data   any              `json:"data"`
				Errors []graphql.Error  `json:"errors"`
			}
			if err := json.Unmarshal(respBody, &respData); err != nil {
				continue
			}

			hasErrors := len(respData.Errors) > 0
			if !hasErrors && errorsOnly {
				continue
			}

			if hasErrors {
				summary.EntriesWithErrors++
				summary.TotalErrors += len(respData.Errors)

				// Per the GraphQL spec: if data is null or absent, it's a full failure
				// (execution didn't start or was completely aborted). If data is present
				// (even empty object), it's a partial failure (some resolvers succeeded).
				// Note: both {"data": null} and {"errors": [...]} (no data key) unmarshal
				// Data as nil, which is correct -- both are full failures.
				isPartial := respData.Data != nil
				isFullFailure := respData.Data == nil

				if isPartial {
					summary.PartialFailures++
				}
				if isFullFailure {
					summary.FullFailures++
				}

				errorGroups = append(errorGroups, graphql.ErrorGroup{
					EntryID:       entryID,
					OperationName: opName,
					Errors:        respData.Errors,
					IsPartial:     isPartial,
					IsFullFailure: isFullFailure,
				})
			} else if !errorsOnly {
				// Include non-error entries when errorsOnly is false
				errorGroups = append(errorGroups, graphql.ErrorGroup{
					EntryID:       entryID,
					OperationName: opName,
				})
			}
		}

		// Build hint
		var hint string
		if summary.EntriesWithErrors > 0 {
			// Count errors per operation and find the most-errored for the hint
			opErrorCount := make(map[string]int)
			var sampleErrorEntryID string
			for _, eg := range errorGroups {
				if eg.OperationName != "" && len(eg.Errors) > 0 {
					opErrorCount[eg.OperationName] += len(eg.Errors)
					if sampleErrorEntryID == "" {
						sampleErrorEntryID = eg.EntryID
					}
				}
			}
			if len(opErrorCount) > 0 {
				// Pick the operation with the most errors (alphabetical tiebreaker for determinism)
				bestOp := ""
				bestCount := 0
				for name, count := range opErrorCount {
					if count > bestCount || (count == bestCount && (bestOp == "" || name < bestOp)) {
						bestOp = name
						bestCount = count
					}
				}
				hint = fmt.Sprintf("Use powhttp_graphql_inspect(operation_name=%q) to see the operation schema and variables.", bestOp)
				if sampleErrorEntryID != "" {
					hint += fmt.Sprintf(" Use powhttp_get_entry(entry_id=%q, include_headers=true) for full request/response details.", sampleErrorEntryID)
				}
			}
		} else if summary.EntriesChecked > 0 {
			hint = "No GraphQL errors found in the checked entries. Use powhttp_graphql_inspect() to examine operation schemas, or powhttp_graphql_operations() to survey all operations."
		}

		return nil, GraphQLErrorsOutput{
			ErrorGroups: errorGroups,
			Summary:     summary,
			Hint:        hint,
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// graphqlSearchLimit is the internal search breadth for finding GraphQL entries.
// Larger than maxEntries to ensure we find target operations even in busy APIs.
const graphqlSearchLimit = 500

// graphqlParseCacheEntry stores a cached GraphQL parse result for an entry.
type graphqlParseCacheEntry struct {
	result *graphql.ParseResult // nil if not a valid GraphQL body
	ok     bool                 // true if this entry is GraphQL
}

// parseGraphQLEntry fetches, decodes, and parses a GraphQL request body,
// caching the result on Deps so subsequent calls for the same entry are free.
// Returns (parseResult, true) for GraphQL entries, (nil, false) otherwise.
func parseGraphQLEntry(ctx context.Context, d *Deps, sessionID, entryID string) (*graphql.ParseResult, bool) {
	if v, ok := d.GraphQLParseCache.Load(entryID); ok {
		e := v.(*graphqlParseCacheEntry)
		return e.result, e.ok
	}

	notGQL := &graphqlParseCacheEntry{}

	entry, err := d.FetchEntry(ctx, sessionID, entryID)
	if err != nil {
		d.GraphQLParseCache.Store(entryID, notGQL)
		return nil, false
	}

	body, ct, err := d.DecodeBody(entry, "request")
	if err != nil || body == nil || !contenttype.IsJSON(ct) {
		d.GraphQLParseCache.Store(entryID, notGQL)
		return nil, false
	}

	if !graphql.IsGraphQLBody(body) {
		d.GraphQLParseCache.Store(entryID, notGQL)
		return nil, false
	}

	pr, err := graphql.ParseRequestBody(body)
	if err != nil {
		d.GraphQLParseCache.Store(entryID, notGQL)
		return nil, false
	}

	cached := &graphqlParseCacheEntry{result: pr, ok: true}
	d.GraphQLParseCache.Store(entryID, cached)
	return pr, true
}

// resolveGraphQLEntryIDs returns entry IDs either from the provided list or by
// searching for GraphQL entries. When operationName is provided, filters results
// to only entries containing that operation (fetches and parses bodies to verify).
// The host parameter scopes the search to a specific host (empty = all hosts).
func resolveGraphQLEntryIDs(ctx context.Context, d *Deps, sessionID string, entryIDs []string, operationName string, host string, maxEntries int) ([]string, error) {
	if len(entryIDs) > 0 {
		if len(entryIDs) > maxEntries {
			entryIDs = entryIDs[:maxEntries]
		}
		return entryIDs, nil
	}

	// Search all POST requests; parseGraphQLEntry handles body validation.
	searchResp, err := d.Search.Search(ctx, &types.SearchRequest{
		SessionID: sessionID,
		Filters: &types.SearchFilters{
			Method: "POST",
			Host:   host,
		},
		Limit: graphqlSearchLimit,
	})
	if err != nil {
		return nil, WrapPowHTTPError(err)
	}

	// Filter results: validate GraphQL bodies and optionally match operation name.
	// Uses the shared parse cache so repeated calls for the same entries are free.
	ids := make([]string, 0, maxEntries)
	for _, r := range searchResp.Results {
		if len(ids) >= maxEntries {
			break
		}

		pr, ok := parseGraphQLEntry(ctx, d, sessionID, r.Summary.EntryID)
		if !ok {
			continue
		}

		// When operation name is specified, filter by parsed operations
		if operationName != "" {
			found := false
			for _, op := range pr.Operations {
				if op.Name == operationName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		ids = append(ids, r.Summary.EntryID)
	}

	return ids, nil
}

// responseHasGraphQLErrors checks if a response body contains a non-empty "errors" array.
func responseHasGraphQLErrors(body []byte) bool {
	var resp struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	return len(resp.Errors) > 0
}

// formatEntryIDs formats entry IDs for display in hints.
func formatEntryIDs(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
