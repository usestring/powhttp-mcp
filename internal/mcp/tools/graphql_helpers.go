package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/graphql"
	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// GraphQLAnalysis holds the cached result of a full inspect+errors analysis
// for a single operation. Populated by ToolInspectGraphQLOperation and read
// by GraphQL resource handlers.
type GraphQLAnalysis struct {
	SessionID      string
	OperationName  string
	Query          string              // raw query string
	VariablesSchema *jsonschema.Schema // inferred variables schema
	ResponseSchema *jsonschema.Schema  // inferred response schema
	FieldStats     []js.FieldStat      // response field statistics
	ErrorGroups    []graphql.ErrorGroup
	ErrorSummary   graphql.ErrorSummary
	EntryIDs       []string
	EntriesMatched int
}

// graphqlAnalysisCacheKey returns the cache key for a GraphQL analysis.
func graphqlAnalysisCacheKey(sessionID, operationName string) string {
	return sessionID + ":" + operationName
}

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
		Limit: d.Config.MaxSearchResults,
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

// responseGraphQLErrorsByIndex returns a map from batch index to whether that
// individual response contains GraphQL errors. For non-batched responses,
// returns a single entry at index 0.
func responseGraphQLErrorsByIndex(body []byte) map[int]bool {
	body = trimJSONSpace(body)
	if len(body) == 0 {
		return nil
	}

	if body[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil
		}
		result := make(map[int]bool, len(arr))
		for i, item := range arr {
			result[i] = singleResponseHasErrors(item)
		}
		return result
	}

	return map[int]bool{0: singleResponseHasErrors(body)}
}

// singleResponseHasErrors checks if a single GraphQL response object has errors.
func singleResponseHasErrors(body []byte) bool {
	var resp struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	return len(resp.Errors) > 0
}

// trimJSONSpace trims ASCII whitespace from both ends.
func trimJSONSpace(b []byte) []byte {
	start := 0
	for start < len(b) && b[start] <= ' ' {
		start++
	}
	end := len(b)
	for end > start && b[end-1] <= ' ' {
		end--
	}
	return b[start:end]
}

// formatEntryIDs formats entry IDs for display in hints.
func formatEntryIDs(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
