package tools

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/graphql"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ---------------------------------------------------------------------------
// Input / Output types
// ---------------------------------------------------------------------------

// SurveyGraphQLInput is the input for powhttp_survey_graphql.
type SurveyGraphQLInput struct {
	SessionID     string                  `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Scope         *GraphQLOperationsScope `json:"scope,omitempty" jsonschema:"Filtering scope"`
	OperationType string                  `json:"operation_type,omitempty" jsonschema:"Filter by operation type. Valid values: query, mutation, subscription"`
	Limit         int                     `json:"limit,omitempty" jsonschema:"Max clusters to return (default: 50)"`
	Offset        int                     `json:"offset,omitempty" jsonschema:"Pagination offset"`
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

// SurveyGraphQLOutput is the output for powhttp_survey_graphql.
type SurveyGraphQLOutput struct {
	Clusters []graphql.OperationCluster `json:"operation_clusters,omitzero"`
	Summary  graphql.TrafficSummary     `json:"traffic_summary"`
	Hint     string                     `json:"hint,omitempty"`
}

// ---------------------------------------------------------------------------
// Tool: powhttp_survey_graphql
// ---------------------------------------------------------------------------

// ToolSurveyGraphQL clusters GraphQL traffic by operation name and type.
func ToolSurveyGraphQL(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input SurveyGraphQLInput) (*sdkmcp.CallToolResult, SurveyGraphQLOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input SurveyGraphQLInput) (*sdkmcp.CallToolResult, SurveyGraphQLOutput, error) {
		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, SurveyGraphQLOutput{}, err
		}

		if input.OperationType != "" {
			switch input.OperationType {
			case "query", "mutation", "subscription":
			default:
				return nil, SurveyGraphQLOutput{}, ErrInvalidInput("operation_type must be 'query', 'mutation', or 'subscription'")
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
			return nil, SurveyGraphQLOutput{}, WrapPowHTTPError(err)
		}

		if len(searchResp.Results) == 0 {
			return textResult("No POST requests found. Try `extract_endpoints()` to see what endpoints exist, or `search_entries(filters={method: \"POST\"})` to find POST traffic.\n"),
				SurveyGraphQLOutput{}, nil
		}

		// Parse GraphQL bodies from discovered entries using the shared cache.
		type parsedEntry struct {
			entryID    string
			result     *graphql.ParseResult
			host       string
			errByIndex map[int]bool // per-operation error status (batch index -> has errors)
		}

		var parsed []parsedEntry
		hosts := make(map[string]bool)

		for _, sr := range searchResp.Results {
			entryID := sr.Summary.EntryID

			pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID)
			if !ok {
				continue
			}

			// Check for GraphQL errors in response, per batch index
			var errByIndex map[int]bool
			entry, err := d.FetchEntry(ctx, sessionID, entryID) // already cached
			if err == nil {
				respBody, respCT, respErr := d.DecodeBody(entry, "response")
				if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
					errByIndex = responseGraphQLErrorsByIndex(respBody)
				}

				if u, err := url.Parse(entry.URL); err == nil {
					hosts[u.Host] = true
				}
			}

			parsed = append(parsed, parsedEntry{
				entryID:    entryID,
				result:     pr,
				host:       sr.Summary.URL,
				errByIndex: errByIndex,
			})
		}

		if len(parsed) == 0 {
			return textResult("Found POST requests but none contained valid GraphQL bodies (JSON with a \"query\" field). Use `get_entry(entry_id=..., body_mode=\"preview\")` to inspect raw bodies, or `extract_endpoints()` to see endpoint patterns.\n"),
				SurveyGraphQLOutput{}, nil
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
			for opIdx, op := range pe.result.Operations {
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
				if pe.errByIndex[opIdx] {
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

		if len(clusters) == 0 && input.OperationType != "" {
			return textResult(fmt.Sprintf("No %s operations found. Remove the operation_type filter to see all operations.\n", input.OperationType)),
				SurveyGraphQLOutput{}, nil
		}

		out := SurveyGraphQLOutput{
			Clusters: clusters,
			Summary:  summary,
		}
		return hybridResult(renderOperationsText(clusters, summary), out), SurveyGraphQLOutput{}, nil
	}
}
