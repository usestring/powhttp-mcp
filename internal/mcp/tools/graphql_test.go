package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/pkg/graphql"
)

func TestResponseGraphQLErrorsByIndex_SingleWithErrors(t *testing.T) {
	body := []byte(`{"data": null, "errors": [{"message": "not found"}]}`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: true}, result)
}

func TestResponseGraphQLErrorsByIndex_SingleNoErrors(t *testing.T) {
	body := []byte(`{"data": {"user": {"id": "1"}}}`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: false}, result)
}

func TestResponseGraphQLErrorsByIndex_BatchedMixed(t *testing.T) {
	// Batched response: first op succeeds, second has errors, third succeeds
	body := []byte(`[
		{"data": {"user": {"id": "1"}}},
		{"data": null, "errors": [{"message": "forbidden"}]},
		{"data": {"posts": []}}
	]`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: false, 1: true, 2: false}, result)
}

func TestResponseGraphQLErrorsByIndex_BatchedAllErrors(t *testing.T) {
	body := []byte(`[
		{"errors": [{"message": "err1"}]},
		{"errors": [{"message": "err2"}]}
	]`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: true, 1: true}, result)
}

func TestResponseGraphQLErrorsByIndex_BatchedNoErrors(t *testing.T) {
	body := []byte(`[
		{"data": {"a": 1}},
		{"data": {"b": 2}}
	]`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: false, 1: false}, result)
}

func TestResponseGraphQLErrorsByIndex_EmptyBody(t *testing.T) {
	assert.Nil(t, responseGraphQLErrorsByIndex([]byte("")))
	assert.Nil(t, responseGraphQLErrorsByIndex([]byte("  ")))
}

func TestResponseGraphQLErrorsByIndex_InvalidJSON(t *testing.T) {
	// Non-JSON falls through to single-response path; unmarshal fails → no errors
	result := responseGraphQLErrorsByIndex([]byte("not json"))
	assert.Equal(t, map[int]bool{0: false}, result)
}

func TestResponseGraphQLErrorsByIndex_PartialFailure(t *testing.T) {
	// Partial failure: data is present but errors also exist
	body := []byte(`{"data": {"user": {"id": "1"}}, "errors": [{"message": "partial", "path": ["user", "email"]}]}`)
	result := responseGraphQLErrorsByIndex(body)
	assert.Equal(t, map[int]bool{0: true}, result)
}

// ---------------------------------------------------------------------------
// varAccumulator tests
// ---------------------------------------------------------------------------

func TestVarAccumulator_ScalarTypes(t *testing.T) {
	va := newVarAccumulator()
	va.add(map[string]any{"name": "alice", "age": float64(30), "active": true})
	va.add(map[string]any{"name": "bob", "age": float64(25), "active": false})
	va.add(map[string]any{"name": "alice", "age": float64(30), "active": true})

	dist := va.toDistribution(5)
	require.NotNil(t, dist)

	// name: string, 2 unique values, "alice" appears 2x
	assert.Equal(t, "string", dist["name"].Type)
	assert.Equal(t, 2, dist["name"].UniqueCount)
	assert.Equal(t, 0, dist["name"].NullCount)
	require.Len(t, dist["name"].TopValues, 2)
	assert.Equal(t, "alice", dist["name"].TopValues[0].Value)
	assert.Equal(t, 2, dist["name"].TopValues[0].Count)
	assert.Equal(t, "bob", dist["name"].TopValues[1].Value)
	assert.Equal(t, 1, dist["name"].TopValues[1].Count)

	// age: number, 2 unique values
	assert.Equal(t, "number", dist["age"].Type)
	assert.Equal(t, 2, dist["age"].UniqueCount)
	require.Len(t, dist["age"].TopValues, 2)
	assert.Equal(t, float64(30), dist["age"].TopValues[0].Value)
	assert.Equal(t, 2, dist["age"].TopValues[0].Count)

	// active: boolean
	assert.Equal(t, "boolean", dist["active"].Type)
	assert.Equal(t, 2, dist["active"].UniqueCount)
}

func TestVarAccumulator_NullValues(t *testing.T) {
	va := newVarAccumulator()
	va.add(map[string]any{"id": "abc"})
	va.add(map[string]any{"id": nil})
	va.add(map[string]any{"id": nil})

	dist := va.toDistribution(5)
	require.NotNil(t, dist)

	assert.Equal(t, "string", dist["id"].Type) // dominant non-null type
	assert.Equal(t, 1, dist["id"].UniqueCount)
	assert.Equal(t, 2, dist["id"].NullCount)
	require.Len(t, dist["id"].TopValues, 1)
	assert.Equal(t, "abc", dist["id"].TopValues[0].Value)
}

func TestVarAccumulator_AllNull(t *testing.T) {
	va := newVarAccumulator()
	va.add(map[string]any{"x": nil})
	va.add(map[string]any{"x": nil})

	dist := va.toDistribution(5)
	require.NotNil(t, dist)

	assert.Equal(t, "null", dist["x"].Type)
	assert.Equal(t, 0, dist["x"].UniqueCount)
	assert.Equal(t, 2, dist["x"].NullCount)
	assert.Empty(t, dist["x"].TopValues)
}

func TestVarAccumulator_ComplexTypes(t *testing.T) {
	va := newVarAccumulator()
	va.add(map[string]any{"filter": map[string]any{"status": "active"}})
	va.add(map[string]any{"filter": map[string]any{"status": "inactive"}})
	va.add(map[string]any{"tags": []any{"a", "b"}})

	dist := va.toDistribution(5)
	require.NotNil(t, dist)

	// Complex types get unique_count but no top_values
	assert.Equal(t, "object", dist["filter"].Type)
	assert.Equal(t, 2, dist["filter"].UniqueCount)
	assert.Empty(t, dist["filter"].TopValues)

	assert.Equal(t, "array", dist["tags"].Type)
	assert.Equal(t, 1, dist["tags"].UniqueCount)
	assert.Empty(t, dist["tags"].TopValues)
}

func TestVarAccumulator_TopValuesCap(t *testing.T) {
	va := newVarAccumulator()
	for i := 0; i < 10; i++ {
		va.add(map[string]any{"letter": string(rune('a' + i))})
	}

	dist := va.toDistribution(3)
	require.NotNil(t, dist)

	assert.Equal(t, 10, dist["letter"].UniqueCount)
	assert.Len(t, dist["letter"].TopValues, 3) // capped at 3
}

func TestVarAccumulator_Empty(t *testing.T) {
	va := newVarAccumulator()
	assert.Nil(t, va.toDistribution(5))
}

func TestVarAccumulator_NonMapVariables(t *testing.T) {
	va := newVarAccumulator()
	va.add("not a map")
	va.add(nil)
	assert.Nil(t, va.toDistribution(5))
}

func TestVarAccumulator_MixedPresence(t *testing.T) {
	// Variables that appear in some entries but not others
	va := newVarAccumulator()
	va.add(map[string]any{"a": "x", "b": float64(1)})
	va.add(map[string]any{"a": "y"})
	va.add(map[string]any{"b": float64(2)})

	dist := va.toDistribution(5)
	require.NotNil(t, dist)

	assert.Equal(t, 2, dist["a"].UniqueCount)
	assert.Equal(t, 2, dist["b"].UniqueCount)
}

// ---------------------------------------------------------------------------
// Fragment warning detection tests
// ---------------------------------------------------------------------------

func TestDetectFragmentWarnings_TypenameOnly(t *testing.T) {
	body := []byte(`{"data": {"nav": {"items": [{"__typename": "Level5"}]}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	require.Len(t, warnings, 1)
	assert.Equal(t, "data.nav.items[0]", warnings[0].Path)
	assert.Equal(t, "Level5", warnings[0].Typename)
	assert.Contains(t, warnings[0].Message, "... on Level5 { ... }")
}

func TestDetectFragmentWarnings_TypenameWithNullSiblings(t *testing.T) {
	body := []byte(`{"data": {"hero": {"__typename": "Droid", "name": null, "primaryFunction": null}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	require.Len(t, warnings, 1)
	assert.Equal(t, "data.hero", warnings[0].Path)
	assert.Equal(t, "Droid", warnings[0].Typename)
}

func TestDetectFragmentWarnings_TypenameWithRealData(t *testing.T) {
	// __typename present but siblings have real values — no warning
	body := []byte(`{"data": {"hero": {"__typename": "Human", "name": "Luke", "height": 1.72}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	assert.Empty(t, warnings)
}

func TestDetectFragmentWarnings_NoTypename(t *testing.T) {
	body := []byte(`{"data": {"user": {"id": "1", "name": "Alice"}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	assert.Empty(t, warnings)
}

func TestDetectFragmentWarnings_NestedArray(t *testing.T) {
	body := []byte(`{"data": {"search": {"results": [
		{"__typename": "User", "name": "Bob"},
		{"__typename": "Post"},
		{"__typename": "Comment"}
	]}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	require.Len(t, warnings, 2)
	// Post and Comment have only __typename
	typenames := []string{warnings[0].Typename, warnings[1].Typename}
	assert.Contains(t, typenames, "Post")
	assert.Contains(t, typenames, "Comment")
}

func TestDetectFragmentWarnings_Deduplication(t *testing.T) {
	// Same path+typename across multiple entries should be deduplicated
	body1 := []byte(`{"data": {"item": {"__typename": "TypeA"}}}`)
	body2 := []byte(`{"data": {"item": {"__typename": "TypeA"}}}`)
	warnings := detectFragmentWarnings([][]byte{body1, body2})
	require.Len(t, warnings, 1)
}

func TestDetectFragmentWarnings_MultipleEntries(t *testing.T) {
	// Different types across entries
	body1 := []byte(`{"data": {"item": {"__typename": "TypeA"}}}`)
	body2 := []byte(`{"data": {"item": {"__typename": "TypeB"}}}`)
	warnings := detectFragmentWarnings([][]byte{body1, body2})
	require.Len(t, warnings, 2)
}

func TestDetectFragmentWarnings_InvalidJSON(t *testing.T) {
	warnings := detectFragmentWarnings([][]byte{[]byte("not json")})
	assert.Empty(t, warnings)
}

func TestDetectFragmentWarnings_EmptyBodies(t *testing.T) {
	warnings := detectFragmentWarnings(nil)
	assert.Empty(t, warnings)
}

func TestDetectFragmentWarnings_MixedNullAndNonNull(t *testing.T) {
	// One sibling is null, one is not — no warning
	body := []byte(`{"data": {"hero": {"__typename": "Human", "name": "Luke", "age": null}}}`)
	warnings := detectFragmentWarnings([][]byte{body})
	assert.Empty(t, warnings)
}

// ---------------------------------------------------------------------------
// Fragment coverage tests
// ---------------------------------------------------------------------------

func TestComputeFragmentCoverage_MatchedAndUnmatched(t *testing.T) {
	query := `
		query GetHero {
			hero {
				__typename
				... on Human { height }
				... on Droid { primaryFunction }
			}
		}
	`
	// Response has Human and Wookiee types, no Droid
	body := []byte(`{"data": {"hero": {"__typename": "Human", "height": 1.72}}}`)
	body2 := []byte(`{"data": {"hero": {"__typename": "Wookiee"}}}`)

	cov := computeFragmentCoverage(query, [][]byte{body, body2})
	require.NotNil(t, cov)

	// Should have 2 inline fragments (Human, Droid)
	assert.Len(t, cov.Fragments, 2)

	// Should see Human (has fragment) and Wookiee (no fragment)
	require.Len(t, cov.TypenamesSeen, 2)
	humanSeen := findTypenameSeen(cov.TypenamesSeen, "Human")
	require.NotNil(t, humanSeen)
	assert.True(t, humanSeen.HasFragment)

	wookieeSeen := findTypenameSeen(cov.TypenamesSeen, "Wookiee")
	require.NotNil(t, wookieeSeen)
	assert.False(t, wookieeSeen.HasFragment)

	// Wookiee is unmatched
	require.Len(t, cov.UnmatchedTypes, 1)
	assert.Equal(t, "Wookiee", cov.UnmatchedTypes[0].Typename)
	assert.Contains(t, cov.UnmatchedTypes[0].Message, "... on Wookiee")
}

func TestComputeFragmentCoverage_UnusedFragment(t *testing.T) {
	query := `
		query Search {
			search { ...UserFields ...PostFields }
		}
		fragment UserFields on User { name }
		fragment PostFields on Post { title }
	`
	// Only User appears in response
	body := []byte(`{"data": {"search": [{"__typename": "User", "name": "Alice"}]}}`)

	cov := computeFragmentCoverage(query, [][]byte{body})
	require.NotNil(t, cov)

	// PostFields is unused (Post never appeared)
	assert.Contains(t, cov.UnusedFragments, "PostFields")
	assert.NotContains(t, cov.UnusedFragments, "UserFields")
}

func TestComputeFragmentCoverage_NoFragmentsNoTypenames(t *testing.T) {
	query := `query GetUser { user { id name } }`
	body := []byte(`{"data": {"user": {"id": "1", "name": "Alice"}}}`)

	cov := computeFragmentCoverage(query, [][]byte{body})
	assert.Nil(t, cov) // nothing to report
}

func TestComputeFragmentCoverage_EmptyBodies(t *testing.T) {
	query := `query Foo { ... on Bar { x } }`
	cov := computeFragmentCoverage(query, nil)
	// Has a fragment but no bodies — still returns coverage with the fragment listed
	require.NotNil(t, cov)
	assert.Len(t, cov.Fragments, 1)
	assert.Empty(t, cov.TypenamesSeen)
}

func findTypenameSeen(seen []graphql.TypenameSeen, typename string) *graphql.TypenameSeen {
	for i := range seen {
		if seen[i].Typename == typename {
			return &seen[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Response variant detection tests
// ---------------------------------------------------------------------------

func TestComputeResponseVariants_DifferentShapes(t *testing.T) {
	shapes := []entryShape{
		{entryID: "e1", shapeKey: "hero.name,hero.height", shapeKeys: []string{"hero.name", "hero.height"}, variables: map[string]any{"type": "human"}},
		{entryID: "e2", shapeKey: "hero.name,hero.height", shapeKeys: []string{"hero.name", "hero.height"}, variables: map[string]any{"type": "human"}},
		{entryID: "e3", shapeKey: "hero.name,hero.primaryFunction", shapeKeys: []string{"hero.name", "hero.primaryFunction"}, variables: map[string]any{"type": "droid"}},
	}

	rv := computeResponseVariants(shapes)
	require.NotNil(t, rv)
	assert.Equal(t, "type", rv.DiscriminatingVariable)
	require.Len(t, rv.Variants, 2)

	// First group (human) has 2 entries
	assert.Equal(t, 2, rv.Variants[0].EntryCount)
	// Second group (droid) has 1 entry
	assert.Equal(t, 1, rv.Variants[1].EntryCount)
}

func TestComputeResponseVariants_SameShape(t *testing.T) {
	shapes := []entryShape{
		{entryID: "e1", shapeKey: "hero", shapeKeys: []string{"hero"}, variables: map[string]any{"id": "1"}},
		{entryID: "e2", shapeKey: "hero", shapeKeys: []string{"hero"}, variables: map[string]any{"id": "2"}},
	}

	rv := computeResponseVariants(shapes)
	assert.Nil(t, rv) // all same shape, no variants
}

func TestComputeResponseVariants_TooFewEntries(t *testing.T) {
	shapes := []entryShape{
		{entryID: "e1", shapeKey: "hero", shapeKeys: []string{"hero"}, variables: map[string]any{"id": "1"}},
	}

	rv := computeResponseVariants(shapes)
	assert.Nil(t, rv) // need at least 2
}

func TestComputeResponseVariants_NoVariables(t *testing.T) {
	shapes := []entryShape{
		{entryID: "e1", shapeKey: "a,b", shapeKeys: []string{"a", "b"}, variables: nil},
		{entryID: "e2", shapeKey: "c,d", shapeKeys: []string{"c", "d"}, variables: nil},
	}

	rv := computeResponseVariants(shapes)
	require.NotNil(t, rv) // different shapes detected
	assert.Equal(t, "", rv.DiscriminatingVariable) // no variables to discriminate
	assert.Len(t, rv.Variants, 2)
}

func TestResponseShapeFingerprint(t *testing.T) {
	body := []byte(`{"data": {"hero": {"name": "Luke", "height": 1.72}, "villain": {"name": "Vader"}}}`)
	fp, keys := responseShapeFingerprint(body)
	assert.NotEmpty(t, fp)
	assert.Contains(t, keys, "hero")
	assert.Contains(t, keys, "villain")
}

func TestResponseShapeFingerprint_NoData(t *testing.T) {
	body := []byte(`{"errors": [{"message": "oops"}]}`)
	fp, keys := responseShapeFingerprint(body)
	assert.Empty(t, fp)
	assert.Nil(t, keys)
}

func TestComputeResponseVariants_MultipleDiscriminators(t *testing.T) {
	// Two variables, but only "class" separates the groups
	shapes := []entryShape{
		{entryID: "e1", shapeKey: "a", shapeKeys: []string{"a"}, variables: map[string]any{"class": "L2", "locale": "en"}},
		{entryID: "e2", shapeKey: "a", shapeKeys: []string{"a"}, variables: map[string]any{"class": "L2", "locale": "fr"}},
		{entryID: "e3", shapeKey: "b", shapeKeys: []string{"b"}, variables: map[string]any{"class": "L4", "locale": "en"}},
	}

	rv := computeResponseVariants(shapes)
	require.NotNil(t, rv)
	assert.Equal(t, "class", rv.DiscriminatingVariable)
}

func TestVarAccumulator_IntegrationWithOperationCluster(t *testing.T) {
	// Verify the type works correctly in OperationCluster
	dist := map[string]graphql.VariableDistribution{
		"id": {
			Type:        "string",
			TopValues:   []graphql.ValueCount{{Value: "abc", Count: 3}},
			UniqueCount: 1,
		},
	}
	cluster := graphql.OperationCluster{
		Name:            "GetUser",
		Type:            "query",
		Count:           3,
		VariableSummary: dist,
	}
	assert.Equal(t, "string", cluster.VariableSummary["id"].Type)
	assert.Equal(t, 1, len(cluster.VariableSummary["id"].TopValues))
}
