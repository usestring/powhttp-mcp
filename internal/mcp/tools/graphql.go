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
	Path         string `json:"path,omitempty" jsonschema:"Override default path detection (disables body-probing fallback). By default, auto-detects POST requests to paths containing 'graphql' or '/gql', then falls back to probing all POST bodies. Set this to search a custom GraphQL endpoint path."`
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
	EntryIDs      []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to inspect. Either entry_ids or operation_name is required."`
	OperationName string   `json:"operation_name,omitempty" jsonschema:"GraphQL operation name to find and inspect. Either entry_ids or operation_name is required."`
	IncludeQuery  *bool    `json:"include_query,omitempty" jsonschema:"Include raw query string in output (default: true)"`
	MaxEntries    int      `json:"max_entries,omitempty" jsonschema:"Max entries to inspect (default: 20, max: 100)"`
}

// GraphQLInspectOutput is the output for powhttp_graphql_inspect.
type GraphQLInspectOutput struct {
	Operations      []graphql.InspectedOperation `json:"operations"`
	EntriesChecked  int                          `json:"entries_checked"`
	EntriesMatched  int                          `json:"entries_matched"`
	Hint            string                       `json:"hint,omitempty"`
}

// GraphQLErrorsInput is the input for powhttp_graphql_errors.
type GraphQLErrorsInput struct {
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs      []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to check. Either entry_ids or operation_name is required."`
	OperationName string   `json:"operation_name,omitempty" jsonschema:"GraphQL operation name to find and check. Either entry_ids or operation_name is required."`
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

		// Build search filters for GraphQL traffic.
		// Strategy: first try path-based detection (fast), then fall back to
		// body-based probing (catches custom paths like /api, /v1/data, etc.).
		searchFilters := &types.SearchFilters{
			Method: "POST",
		}

		explicitPath := ""
		if input.Scope != nil {
			if input.Scope.Host != "" {
				searchFilters.Host = input.Scope.Host
			}
			if input.Scope.Path != "" {
				explicitPath = input.Scope.Path
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

		// Phase 1: path-based search (or explicit path)
		if explicitPath != "" {
			searchFilters.PathContains = explicitPath
		} else {
			searchFilters.PathContains = "graphql"
		}

		searchResp, err := d.Search.Search(ctx, &types.SearchRequest{
			SessionID: sessionID,
			Filters:   searchFilters,
			Limit:     graphqlSearchLimit,
		})
		if err != nil {
			return nil, GraphQLOperationsOutput{}, WrapPowHTTPError(err)
		}

		// Phase 1b: also try /gql path if no explicit path and "graphql" found nothing
		if len(searchResp.Results) == 0 && explicitPath == "" {
			gqlFilters := *searchFilters
			gqlFilters.PathContains = "gql"
			gqlResp, err := d.Search.Search(ctx, &types.SearchRequest{
				SessionID: sessionID,
				Filters:   &gqlFilters,
				Limit:     graphqlSearchLimit,
			})
			if err == nil && len(gqlResp.Results) > 0 {
				searchResp = gqlResp
			}
		}

		// Phase 2: if path-based search found nothing and no explicit path,
		// broaden to all POST requests and probe bodies for GraphQL structure.
		usedBodyProbing := false
		if len(searchResp.Results) == 0 && explicitPath == "" {
			broadFilters := *searchFilters
			broadFilters.PathContains = ""
			broadResp, err := d.Search.Search(ctx, &types.SearchRequest{
				SessionID: sessionID,
				Filters:   &broadFilters,
				Limit:     graphqlSearchLimit,
			})
			if err == nil && len(broadResp.Results) > 0 {
				searchResp = broadResp
				usedBodyProbing = true
			}
		}

		if len(searchResp.Results) == 0 {
			hint := "No GraphQL traffic found."
			if explicitPath != "" {
				hint += fmt.Sprintf(" No POST requests matched path %q. Try powhttp_search_entries(filters={method: \"POST\"}) to see what POST traffic exists, or remove the path override to use auto-detection.", explicitPath)
			} else {
				hint += " Searched POST requests with path containing \"graphql\" or \"gql\", then probed all POST bodies. No GraphQL request bodies (JSON with a \"query\" field) were found. Try powhttp_extract_endpoints() to see what endpoints exist, or powhttp_search_entries(filters={method: \"POST\"}) to find POST traffic."
			}
			return nil, GraphQLOperationsOutput{Hint: hint}, nil
		}

		// Parse GraphQL bodies from discovered entries.
		// When body probing is active, validate each body with IsGraphQLBody
		// before attempting full parse.
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
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				continue
			}

			body, ct, err := d.DecodeBody(entry, "request")
			if err != nil || body == nil {
				continue
			}

			if !contenttype.IsJSON(ct) {
				continue
			}

			// When using broad search, quick-check the body structure first
			if usedBodyProbing && !graphql.IsGraphQLBody(body) {
				continue
			}

			pr, err := graphql.ParseRequestBody(body)
			if err != nil {
				continue
			}

			// Check for GraphQL errors in response
			hasErr := false
			respBody, respCT, respErr := d.DecodeBody(entry, "response")
			if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
				hasErr = responseHasGraphQLErrors(respBody)
			}

			// Extract host
			if u, err := url.Parse(entry.URL); err == nil {
				hosts[u.Host] = true
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

			firstOp := clusters[0].Name
			hint = fmt.Sprintf("Found %d operations (%s). Use powhttp_graphql_inspect(operation_name=%q) for schema details, or powhttp_graphql_errors() to find failures.",
				summary.UniqueOps, strings.Join(parts, ", "), firstOp)
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
		entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, input.EntryIDs, input.OperationName, maxEntries)
		if err != nil {
			return nil, GraphQLInspectOutput{}, err
		}

		if len(entryIDs) == 0 {
			hint := fmt.Sprintf("No entries found for operation %q. Run powhttp_graphql_operations() to see all operation names.", input.OperationName)
			return nil, GraphQLInspectOutput{Hint: hint}, nil
		}

		// Inspect each entry
		var operations []graphql.InspectedOperation
		entriesChecked := 0
		entriesMatched := 0

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				continue
			}
			entriesChecked++

			reqBody, reqCT, err := d.DecodeBody(entry, "request")
			if err != nil || reqBody == nil || !contenttype.IsJSON(reqCT) {
				continue
			}

			pr, err := graphql.ParseRequestBody(reqBody)
			if err != nil {
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

				operations = append(operations, inspected)
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
		entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, input.EntryIDs, input.OperationName, maxEntries)
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

			// Get operation name from request
			opName := ""
			reqBody, reqCT, reqErr := d.DecodeBody(entry, "request")
			if reqErr == nil && reqBody != nil && contenttype.IsJSON(reqCT) {
				if pr, err := graphql.ParseRequestBody(reqBody); err == nil && len(pr.Operations) > 0 {
					opName = pr.Operations[0].Name
				}
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
			// Collect unique operation names and entry IDs from error groups
			opNames := make(map[string]bool)
			var sampleErrorEntryID string
			for _, eg := range errorGroups {
				if eg.OperationName != "" && len(eg.Errors) > 0 {
					opNames[eg.OperationName] = true
					if sampleErrorEntryID == "" {
						sampleErrorEntryID = eg.EntryID
					}
				}
			}
			if len(opNames) > 0 {
				names := make([]string, 0, len(opNames))
				for n := range opNames {
					names = append(names, n)
				}
				sort.Strings(names)
				hint = fmt.Sprintf("Use powhttp_graphql_inspect(operation_name=%q) to see the operation schema and variables.", names[0])
				if sampleErrorEntryID != "" {
					hint += fmt.Sprintf(" Use powhttp_get_entry(entry_id=%q, include_headers=true) for full request/response details.", sampleErrorEntryID)
				}
			}
		} else if summary.EntriesChecked > 0 {
			hint = "No GraphQL errors found in the checked entries. Use powhttp_graphql_inspect() to examine operation schemas instead."
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

// resolveGraphQLEntryIDs returns entry IDs either from the provided list or by
// searching for GraphQL entries. When operationName is provided, filters results
// to only entries containing that operation (fetches and parses bodies to verify).
// Uses path-based detection first, then falls back to body probing.
func resolveGraphQLEntryIDs(ctx context.Context, d *Deps, sessionID string, entryIDs []string, operationName string, maxEntries int) ([]string, error) {
	if len(entryIDs) > 0 {
		if len(entryIDs) > maxEntries {
			entryIDs = entryIDs[:maxEntries]
		}
		return entryIDs, nil
	}

	// Use a larger internal search limit to find the target operation
	// even in busy APIs where it may not appear in the first few results.
	searchLimit := graphqlSearchLimit

	// Phase 1: try path-based search ("graphql")
	searchResp, err := d.Search.Search(ctx, &types.SearchRequest{
		SessionID: sessionID,
		Filters: &types.SearchFilters{
			Method:       "POST",
			PathContains: "graphql",
		},
		Limit: searchLimit,
	})
	if err != nil {
		return nil, WrapPowHTTPError(err)
	}

	// Phase 1b: also try "gql" path
	if len(searchResp.Results) == 0 {
		gqlResp, err := d.Search.Search(ctx, &types.SearchRequest{
			SessionID: sessionID,
			Filters: &types.SearchFilters{
				Method:       "POST",
				PathContains: "gql",
			},
			Limit: searchLimit,
		})
		if err != nil {
			return nil, WrapPowHTTPError(err)
		}
		if len(gqlResp.Results) > 0 {
			searchResp = gqlResp
		}
	}

	// Phase 2: if path-based found nothing, broaden to all POST
	if len(searchResp.Results) == 0 {
		broadResp, err := d.Search.Search(ctx, &types.SearchRequest{
			SessionID: sessionID,
			Filters: &types.SearchFilters{
				Method: "POST",
			},
			Limit: searchLimit,
		})
		if err != nil {
			return nil, WrapPowHTTPError(err)
		}
		searchResp = broadResp
	}

	// Filter results: validate GraphQL bodies and optionally match operation name.
	ids := make([]string, 0, maxEntries)
	for _, r := range searchResp.Results {
		if len(ids) >= maxEntries {
			break
		}

		entry, err := d.FetchEntry(ctx, sessionID, r.Summary.EntryID)
		if err != nil {
			continue
		}
		body, ct, err := d.DecodeBody(entry, "request")
		if err != nil || body == nil || !contenttype.IsJSON(ct) {
			continue
		}

		if !graphql.IsGraphQLBody(body) {
			continue
		}

		// When operation name is specified, parse and filter
		if operationName != "" {
			pr, err := graphql.ParseRequestBody(body)
			if err != nil {
				continue
			}
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
