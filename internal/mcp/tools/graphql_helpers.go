package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	SessionID            string
	OperationName        string
	Query                string                                  // raw query string
	VariablesSchema      *jsonschema.Schema                      // inferred variables schema
	VariableDistribution map[string]graphql.VariableDistribution // per-variable value distribution
	ResponseSchema       *jsonschema.Schema                      // inferred response schema
	FieldStats           []js.FieldStat                          // response field statistics
	ErrorGroups          []graphql.ErrorGroup
	ErrorSummary         graphql.ErrorSummary
	EntryIDs             []string
	EntriesMatched       int
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

// ---------------------------------------------------------------------------
// Fragment warning detection
// ---------------------------------------------------------------------------

// detectFragmentWarnings scans a GraphQL response body for objects that suggest
// a missing union/interface fragment. An object is flagged when:
//   - Its only key is __typename, OR
//   - __typename is present and all sibling values are null
//
// Returns a deduplicated list of warnings across all provided response bodies.
func detectFragmentWarnings(bodies [][]byte) []graphql.FragmentWarning {
	seen := make(map[[2]string]bool)
	var warnings []graphql.FragmentWarning

	for _, body := range bodies {
		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			continue
		}
		walkForFragmentWarnings(parsed, "data", seen, &warnings)
	}
	return warnings
}

// walkForFragmentWarnings recursively walks a JSON value looking for objects
// that need fragment spreads. The path parameter tracks the current JSON path.
// It starts from "data" because GraphQL responses nest data under the "data" key.
func walkForFragmentWarnings(v any, path string, seen map[[2]string]bool, out *[]graphql.FragmentWarning) {
	switch val := v.(type) {
	case map[string]any:
		// Check if this is the top-level response — descend into "data"
		if path == "data" {
			if data, ok := val["data"]; ok {
				walkForFragmentWarnings(data, "data", seen, out)
				return
			}
		}

		// Check for __typename-only pattern
		if typename, ok := val["__typename"].(string); ok && typename != "" {
			if needsFragment(val) {
				key := [2]string{path, typename}
				if !seen[key] {
					seen[key] = true
					*out = append(*out, graphql.FragmentWarning{
						Path:     path,
						Typename: typename,
						Message:  fmt.Sprintf("Object at `%s` has only `__typename=%q` — add a `... on %s { ... }` fragment", path, typename, typename),
					})
				}
			}
		}

		// Recurse into child values
		for k, child := range val {
			childPath := path + "." + k
			walkForFragmentWarnings(child, childPath, seen, out)
		}

	case []any:
		for i, item := range val {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			walkForFragmentWarnings(item, itemPath, seen, out)
		}
	}
}

// needsFragment returns true if the object has __typename and either:
// - __typename is the only key, or
// - all sibling values are null.
func needsFragment(obj map[string]any) bool {
	if len(obj) == 1 {
		// Only __typename
		return true
	}
	// Check if all siblings are null
	for k, v := range obj {
		if k == "__typename" {
			continue
		}
		if v != nil {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// __typename collection (for fragment coverage)
// ---------------------------------------------------------------------------

// typenameOccurrence records where a __typename value was found.
type typenameOccurrence struct {
	paths map[string]bool // deduplicated paths
	count int             // total occurrences
}

// collectTypenames scans response bodies for __typename values and returns
// a map of typename -> occurrence info.
func collectTypenames(bodies [][]byte) map[string]*typenameOccurrence {
	result := make(map[string]*typenameOccurrence)
	for _, body := range bodies {
		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			continue
		}
		walkForTypenames(parsed, "data", result)
	}
	return result
}

// walkForTypenames recursively walks a JSON value collecting __typename values.
func walkForTypenames(v any, path string, out map[string]*typenameOccurrence) {
	switch val := v.(type) {
	case map[string]any:
		// Descend into "data" at the top level
		if path == "data" {
			if data, ok := val["data"]; ok {
				walkForTypenames(data, "data", out)
				return
			}
		}

		if typename, ok := val["__typename"].(string); ok && typename != "" {
			occ := out[typename]
			if occ == nil {
				occ = &typenameOccurrence{paths: make(map[string]bool)}
				out[typename] = occ
			}
			occ.count++
			if len(occ.paths) < 5 { // cap example paths
				occ.paths[path] = true
			}
		}

		for k, child := range val {
			walkForTypenames(child, path+"."+k, out)
		}
	case []any:
		for i, item := range val {
			walkForTypenames(item, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

// computeFragmentCoverage cross-references query fragments against response __typename values.
func computeFragmentCoverage(query string, bodies [][]byte) *graphql.FragmentCoverage {
	fragments := graphql.ExtractFragments(query)
	typenames := collectTypenames(bodies)

	if len(fragments) == 0 && len(typenames) == 0 {
		return nil
	}

	// Build set of fragment on-types
	fragmentTypes := make(map[string]bool, len(fragments))
	for _, f := range fragments {
		fragmentTypes[f.OnType] = true
	}

	// Build TypenamesSeen
	var seen []graphql.TypenameSeen
	for tn, occ := range typenames {
		paths := make([]string, 0, len(occ.paths))
		for p := range occ.paths {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		seen = append(seen, graphql.TypenameSeen{
			Typename:    tn,
			Paths:       paths,
			Count:       occ.count,
			HasFragment: fragmentTypes[tn],
		})
	}
	sort.Slice(seen, func(i, j int) bool {
		return seen[i].Typename < seen[j].Typename
	})

	// Build UnmatchedTypes (typenames with no fragment)
	var unmatched []graphql.UnmatchedType
	for _, ts := range seen {
		if !ts.HasFragment {
			unmatched = append(unmatched, graphql.UnmatchedType{
				Typename:     ts.Typename,
				ExamplePaths: ts.Paths,
				Message:      fmt.Sprintf("Type %q seen at %s but has no fragment — add `... on %s { ... }` or `fragment ... on %s { ... }`", ts.Typename, strings.Join(ts.Paths, ", "), ts.Typename, ts.Typename),
			})
		}
	}

	// Build UnusedFragments (fragments whose type never appeared)
	seenTypes := make(map[string]bool, len(typenames))
	for tn := range typenames {
		seenTypes[tn] = true
	}
	var unused []string
	for _, f := range fragments {
		if !seenTypes[f.OnType] && f.Name != "" {
			unused = append(unused, f.Name)
		}
	}

	return &graphql.FragmentCoverage{
		Fragments:       fragments,
		TypenamesSeen:   seen,
		UnmatchedTypes:  unmatched,
		UnusedFragments: unused,
	}
}

// ---------------------------------------------------------------------------
// Response shape variant detection
// ---------------------------------------------------------------------------

// entryShape captures the response shape and variables for a single entry.
type entryShape struct {
	entryID   string
	shapeKey  string   // sorted comma-joined top-level keys under .data
	shapeKeys []string // the actual keys
	variables map[string]any
}

// shapeGroup groups entries that share the same response shape fingerprint.
type shapeGroup struct {
	shapeKeys []string
	entries   []entryShape
}

// computeResponseVariants groups entries by response shape and identifies the
// discriminating variable. Returns nil if all entries have the same shape or
// there are fewer than 2 entries.
func computeResponseVariants(entryShapes []entryShape) *graphql.ResponseVariants {
	if len(entryShapes) < 2 {
		return nil
	}

	// Group by shape fingerprint
	groups := make(map[string]*shapeGroup)
	var groupOrder []string

	for i := range entryShapes {
		es := &entryShapes[i]
		g := groups[es.shapeKey]
		if g == nil {
			g = &shapeGroup{shapeKeys: es.shapeKeys}
			groups[es.shapeKey] = g
			groupOrder = append(groupOrder, es.shapeKey)
		}
		g.entries = append(g.entries, *es)
	}

	if len(groups) <= 1 {
		return nil // all same shape
	}

	// Cap at 10 variants
	if len(groupOrder) > 10 {
		groupOrder = groupOrder[:10]
	}

	// Find discriminating variable via partition heuristic:
	// For each variable, compute how well its values separate the groups.
	// A perfect discriminator assigns all entries in each group the same value,
	// and different groups get different values.
	discVar := findDiscriminatingVariable(groups, groupOrder)

	// Build variants
	variants := make([]graphql.Variant, 0, len(groupOrder))
	for _, key := range groupOrder {
		g := groups[key]
		v := graphql.Variant{
			EntryCount:     len(g.entries),
			ShapeKeys:      g.shapeKeys,
			ExampleEntryID: g.entries[0].entryID,
		}
		// Compute common variable values for this group
		if discVar != "" {
			v.VariableValues = commonVariableValues(g.entries, discVar)
		}
		variants = append(variants, v)
	}

	return &graphql.ResponseVariants{
		DiscriminatingVariable: discVar,
		Variants:               variants,
	}
}

// findDiscriminatingVariable finds the variable that best separates response shape groups.
// Uses normalized mutual information: for each variable, measure how well
// its value distribution partitions entries into the observed shape groups.
func findDiscriminatingVariable(groups map[string]*shapeGroup, groupOrder []string) string {
	// Collect all variable names
	varNames := make(map[string]bool)
	for _, key := range groupOrder {
		for _, es := range groups[key].entries {
			for k := range es.variables {
				varNames[k] = true
			}
		}
	}

	bestVar := ""
	bestScore := -1.0

	for varName := range varNames {
		score := partitionScore(groups, groupOrder, varName)
		if score > bestScore {
			bestScore = score
			bestVar = varName
		}
	}

	// Only report if the variable actually separates groups somewhat
	if bestScore <= 0 {
		return ""
	}

	return bestVar
}

// partitionScore computes how well a variable separates response shape groups.
// Score is the fraction of group pairs where the variable values don't overlap.
// Score of 1.0 means perfect separation; 0.0 means no separation.
func partitionScore(groups map[string]*shapeGroup, groupOrder []string, varName string) float64 {
	// For each group, collect the set of values for this variable
	type valSet map[string]bool
	groupVals := make([]valSet, len(groupOrder))

	for i, key := range groupOrder {
		vs := make(valSet)
		for _, es := range groups[key].entries {
			v := es.variables[varName]
			b, _ := json.Marshal(v)
			vs[string(b)] = true
		}
		groupVals[i] = vs
	}

	// Count group pairs with no overlap
	pairs := 0
	separated := 0
	for i := 0; i < len(groupVals); i++ {
		for j := i + 1; j < len(groupVals); j++ {
			pairs++
			overlap := false
			for v := range groupVals[i] {
				if groupVals[j][v] {
					overlap = true
					break
				}
			}
			if !overlap {
				separated++
			}
		}
	}

	if pairs == 0 {
		return 0
	}
	return float64(separated) / float64(pairs)
}

// commonVariableValues extracts the most common value for the discriminating
// variable in a group. Returns a map with just the discriminating variable
// and its most common value(s).
func commonVariableValues(entries []entryShape, discVar string) map[string]any {
	counts := make(map[string]int)
	for _, es := range entries {
		v := es.variables[discVar]
		b, _ := json.Marshal(v)
		counts[string(b)]++
	}

	// Find the most common value
	bestKey := ""
	bestCount := 0
	for k, c := range counts {
		if c > bestCount {
			bestCount = c
			bestKey = k
		}
	}

	if bestKey == "" {
		return nil
	}

	var val any
	_ = json.Unmarshal([]byte(bestKey), &val)
	return map[string]any{discVar: val}
}

// responseShapeFingerprint computes a fingerprint from the top-level keys
// under the "data" field of a GraphQL JSON response.
func responseShapeFingerprint(body []byte) (string, []string) {
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Data == nil {
		return "", nil
	}

	// Collect all keys recursively from the first level
	keys := make([]string, 0, len(resp.Data))
	for k, v := range resp.Data {
		// Include sub-keys for object values to differentiate shapes
		subKeys := extractDataKeys(v, k, 2)
		keys = append(keys, subKeys...)
	}
	sort.Strings(keys)
	return strings.Join(keys, ","), keys
}

// extractDataKeys extracts key paths from a JSON value up to maxDepth levels.
func extractDataKeys(raw json.RawMessage, prefix string, maxDepth int) []string {
	if maxDepth <= 0 {
		return []string{prefix}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return []string{prefix}
	}

	keys := []string{prefix}
	for k, v := range obj {
		if k == "__typename" {
			continue
		}
		subKeys := extractDataKeys(v, prefix+"."+k, maxDepth-1)
		keys = append(keys, subKeys...)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Variable value distribution accumulator
// ---------------------------------------------------------------------------

// singleVarAccum collects statistics for a single GraphQL variable.
type singleVarAccum struct {
	typeCounts map[string]int // JSON type → count
	values     map[string]int // JSON-encoded value → count
	nullCount  int
}

// varAccumulator collects variable value statistics incrementally across entries.
type varAccumulator struct {
	vars map[string]*singleVarAccum
}

func newVarAccumulator() *varAccumulator {
	return &varAccumulator{vars: make(map[string]*singleVarAccum)}
}

// add incorporates the variables from a single GraphQL operation.
func (va *varAccumulator) add(variables any) {
	m, ok := variables.(map[string]any)
	if !ok {
		return
	}
	for k, v := range m {
		sv := va.vars[k]
		if sv == nil {
			sv = &singleVarAccum{
				typeCounts: make(map[string]int),
				values:     make(map[string]int),
			}
			va.vars[k] = sv
		}

		if v == nil {
			sv.nullCount++
			sv.typeCounts["null"]++
			continue
		}

		var jsonType string
		switch v.(type) {
		case string:
			jsonType = "string"
		case float64:
			jsonType = "number"
		case bool:
			jsonType = "boolean"
		case map[string]any:
			jsonType = "object"
		case []any:
			jsonType = "array"
		default:
			jsonType = "unknown"
		}

		b, _ := json.Marshal(v)
		sv.typeCounts[jsonType]++
		sv.values[string(b)]++
	}
}

// toDistribution converts accumulated stats into a per-variable distribution map.
// maxTopValues caps the number of top values returned for scalar types.
// Complex types (object, array) get unique_count only, no top values.
func (va *varAccumulator) toDistribution(maxTopValues int) map[string]graphql.VariableDistribution {
	if len(va.vars) == 0 {
		return nil
	}
	result := make(map[string]graphql.VariableDistribution, len(va.vars))
	for name, sv := range va.vars {
		// Determine dominant non-null type
		bestType := ""
		bestCount := 0
		for t, c := range sv.typeCounts {
			if t == "null" {
				continue
			}
			if c > bestCount {
				bestType = t
				bestCount = c
			}
		}
		if bestType == "" {
			bestType = "null"
		}

		dist := graphql.VariableDistribution{
			Type:        bestType,
			UniqueCount: len(sv.values),
			NullCount:   sv.nullCount,
		}

		// For scalar types, compute top values sorted by frequency
		isComplex := bestType == "object" || bestType == "array"
		if !isComplex && len(sv.values) > 0 {
			type kv struct {
				key   string
				count int
			}
			pairs := make([]kv, 0, len(sv.values))
			for k, c := range sv.values {
				pairs = append(pairs, kv{k, c})
			}
			sort.Slice(pairs, func(i, j int) bool {
				if pairs[i].count != pairs[j].count {
					return pairs[i].count > pairs[j].count
				}
				return pairs[i].key < pairs[j].key
			})
			top := maxTopValues
			if top > len(pairs) {
				top = len(pairs)
			}
			dist.TopValues = make([]graphql.ValueCount, top)
			for i := 0; i < top; i++ {
				var val any
				_ = json.Unmarshal([]byte(pairs[i].key), &val)
				dist.TopValues[i] = graphql.ValueCount{Value: val, Count: pairs[i].count}
			}
		}

		result[name] = dist
	}
	return result
}
