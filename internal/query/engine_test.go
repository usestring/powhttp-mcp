package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Query_Simple(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"name": "John", "age": 30}`)

	result, err := engine.Query(data, ".name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"John"}, result.Values)
	assert.Equal(t, 1, result.RawCount)
}

func TestEngine_Query_Array(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [{"name": "a"}, {"name": "b"}, {"name": "c"}]}`)

	result, err := engine.Query(data, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b", "c"}, result.Values)
	assert.Equal(t, 3, result.RawCount)
}

func TestEngine_Query_Deduplicate(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [{"name": "a"}, {"name": "a"}, {"name": "b"}]}`)

	result, err := engine.Query(data, ".items[].name", true, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, result.Values)
	assert.Equal(t, 3, result.RawCount)
	assert.Equal(t, 2, len(result.Values))
}

func TestEngine_Query_MaxResults(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [1, 2, 3, 4, 5]}`)

	result, err := engine.Query(data, ".items[]", false, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Values))
	assert.Equal(t, []any{float64(1), float64(2), float64(3)}, result.Values)
}

func TestEngine_Query_NestedPath(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"data": {"contentLayout": {"modules": [{"name": "mod1"}, {"name": "mod2"}]}}}`)

	result, err := engine.Query(data, ".data.contentLayout.modules[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"mod1", "mod2"}, result.Values)
}

func TestEngine_Query_Select(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [{"status": "active", "name": "a"}, {"status": "inactive", "name": "b"}, {"status": "active", "name": "c"}]}`)

	result, err := engine.Query(data, `.items[] | select(.status == "active") | .name`, false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "c"}, result.Values)
}

func TestEngine_Query_InvalidExpression(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"name": "John"}`)

	_, err := engine.Query(data, ".name[", false, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid jq expression")
}

func TestEngine_Query_InvalidJSON(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{invalid json}`)

	_, err := engine.Query(data, ".name", false, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestEngine_Query_NilValues(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [{"name": "a"}, {"noname": "b"}, {"name": "c"}]}`)

	result, err := engine.Query(data, ".items[].name", false, 0)
	require.NoError(t, err)
	// nil values should be skipped
	assert.Equal(t, []any{"a", "c"}, result.Values)
	assert.Equal(t, 2, result.RawCount)
}

func TestEngine_QueryMultiple(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}, {"name": "b"}]}`),
		[]byte(`{"items": [{"name": "c"}, {"name": "d"}]}`),
	}

	result, err := engine.QueryMultiple(dataList, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b", "c", "d"}, result.Values)
	assert.Equal(t, 4, result.RawCount)
}

func TestEngine_QueryMultiple_Deduplicate(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}, {"name": "b"}]}`),
		[]byte(`{"items": [{"name": "b"}, {"name": "c"}]}`),
	}

	result, err := engine.QueryMultiple(dataList, ".items[].name", true, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b", "c"}, result.Values)
	assert.Equal(t, 4, result.RawCount)
}

func TestEngine_QueryMultiple_MaxResults(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [1, 2, 3]}`),
		[]byte(`{"items": [4, 5, 6]}`),
	}

	result, err := engine.QueryMultiple(dataList, ".items[]", false, 4)
	require.NoError(t, err)
	assert.Equal(t, 4, len(result.Values))
}

func TestEngine_QueryMultiple_InvalidJSON(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": ["a"]}`),
		[]byte(`{invalid}`),
		[]byte(`{"items": ["b"]}`),
	}

	result, err := engine.QueryMultiple(dataList, ".items[]", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, result.Values)
	assert.Len(t, result.Errors, 1)
}

func TestEngine_ValidateExpression(t *testing.T) {
	engine := NewEngine()

	assert.NoError(t, engine.ValidateExpression(".name"))
	assert.NoError(t, engine.ValidateExpression(".data.items[].name"))
	assert.NoError(t, engine.ValidateExpression(`.items[] | select(.status == "active")`))

	assert.Error(t, engine.ValidateExpression(".name["))
	assert.Error(t, engine.ValidateExpression("invalid("))
}

func TestEngine_Query_ObjectExtraction(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"products": [{"name": "A", "price": 10}, {"name": "B", "price": 20}]}`)

	result, err := engine.Query(data, `.products[] | {name, price}`, false, 0)
	require.NoError(t, err)
	assert.Len(t, result.Values, 2)

	first := result.Values[0].(map[string]any)
	assert.Equal(t, "A", first["name"])
	assert.Equal(t, float64(10), first["price"])
}

func TestEngine_Query_DeduplicateComplexObjects(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"items": [{"id": 1, "name": "a"}, {"id": 1, "name": "a"}, {"id": 2, "name": "b"}]}`)

	result, err := engine.Query(data, ".items[]", true, 0)
	require.NoError(t, err)
	assert.Len(t, result.Values, 2) // Duplicate objects should be removed
	assert.Equal(t, 3, result.RawCount)
}

func TestEngine_Query_BooleanAndNumbers(t *testing.T) {
	engine := NewEngine()

	data := []byte(`{"values": [true, false, true, 42, 42, 3.14]}`)

	result, err := engine.Query(data, ".values[]", true, 0)
	require.NoError(t, err)
	assert.Len(t, result.Values, 4) // true, false, 42, 3.14
}

func TestEngine_QueryMultipleWithLabels_ErrorContext(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}]}`),
		[]byte(`{"other": "structure"}`), // .items will be null here
		[]byte(`{"items": [{"name": "b"}]}`),
	}
	labels := []string{"entry-1:response", "entry-2:response", "entry-3:response"}

	result, err := engine.QueryMultipleWithLabels(dataList, labels, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, result.Values)
	// Should have an error with label context - the label should be included
	require.Len(t, result.Errors, 1)
	// Error message should start with the label
	assert.True(t, len(result.Errors[0]) > len("entry-2:response"), "error should contain label and message")
	assert.Equal(t, "entry-2:response", result.Errors[0][:len("entry-2:response")])
}

func TestEngine_QueryMultiple_ErrorDeduplication(t *testing.T) {
	engine := NewEngine()

	// Multiple bodies that will cause the same error
	dataList := [][]byte{
		[]byte(`{"other": "a"}`),
		[]byte(`{"other": "b"}`),
		[]byte(`{"other": "c"}`),
	}

	result, err := engine.QueryMultiple(dataList, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Empty(t, result.Values)
	// Each body gets its own labeled error (body[0], body[1], body[2])
	assert.Len(t, result.Errors, 3)
	// Verify labels are present
	assert.True(t, len(result.Errors[0]) > 7 && result.Errors[0][:7] == "body[0]")
	assert.True(t, len(result.Errors[1]) > 7 && result.Errors[1][:7] == "body[1]")
	assert.True(t, len(result.Errors[2]) > 7 && result.Errors[2][:7] == "body[2]")
}

func TestEngine_Query_ReturnsErrors(t *testing.T) {
	engine := NewEngine()

	// Query that produces a runtime error (iterating over null)
	data := []byte(`{"foo": null}`)
	result, err := engine.Query(data, ".foo[]", false, 0)
	require.NoError(t, err) // Query itself succeeds
	assert.Empty(t, result.Values)
	assert.Len(t, result.Errors, 1) // But we get a runtime error in results
}

func TestEngine_QueryMultipleWithLabels_MatchedIndices(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}]}`),          // index 0: has values
		[]byte(`{"other": "no items"}`),               // index 1: no values (null path)
		[]byte(`{"items": [{"name": "b"}]}`),          // index 2: has values
		[]byte(`{"items": []}`),                       // index 3: empty array, no values
	}
	labels := []string{"entry-1:response", "entry-2:response", "entry-3:response", "entry-4:response"}

	result, err := engine.QueryMultipleWithLabels(dataList, labels, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, result.Values)

	// Only indices 0 and 2 should be in MatchedIndices (they produced values)
	assert.ElementsMatch(t, []int{0, 2}, result.MatchedIndices)
}

func TestEngine_QueryMultipleWithLabels_LabelCounts(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}, {"name": "b"}, {"name": "c"}]}`), // 3 values
		[]byte(`{"items": [{"name": "d"}]}`),                               // 1 value
		[]byte(`{"items": [{"name": "e"}, {"name": "f"}]}`),                // 2 values
	}
	labels := []string{"entry-1:response", "entry-2:response", "entry-3:response"}

	result, err := engine.QueryMultipleWithLabels(dataList, labels, ".items[].name", false, 0)
	require.NoError(t, err)
	assert.Equal(t, 6, result.RawCount)

	// Check per-label counts
	assert.Equal(t, 3, result.LabelCounts["entry-1:response"])
	assert.Equal(t, 1, result.LabelCounts["entry-2:response"])
	assert.Equal(t, 2, result.LabelCounts["entry-3:response"])
}

func TestEngine_QueryMultipleWithLabels_LabelCounts_WithDedup(t *testing.T) {
	engine := NewEngine()

	dataList := [][]byte{
		[]byte(`{"items": [{"name": "a"}, {"name": "a"}]}`), // 2 raw, 1 unique
		[]byte(`{"items": [{"name": "a"}, {"name": "b"}]}`), // 2 raw, 1 new unique (b)
	}
	labels := []string{"entry-1:response", "entry-2:response"}

	result, err := engine.QueryMultipleWithLabels(dataList, labels, ".items[].name", true, 0)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, result.Values)
	assert.Equal(t, 4, result.RawCount) // Total raw count

	// LabelCounts still tracks all values found (before dedup)
	assert.Equal(t, 2, result.LabelCounts["entry-1:response"])
	assert.Equal(t, 2, result.LabelCounts["entry-2:response"])
}

func TestEngine_QueryMultipleWithLabels_MatchedIndices_TwoDigits(t *testing.T) {
	engine := NewEngine()

	// Create 15 bodies, with values at indices 0, 5, and 12
	dataList := make([][]byte, 15)
	labels := make([]string, 15)
	for i := 0; i < 15; i++ {
		if i == 0 || i == 5 || i == 12 {
			dataList[i] = []byte(`{"value": "found"}`)
		} else {
			dataList[i] = []byte(`{"other": "nothing"}`)
		}
		labels[i] = "entry-" + string(rune('A'+i)) + ":response"
	}

	result, err := engine.QueryMultipleWithLabels(dataList, labels, ".value", false, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Values))
	assert.ElementsMatch(t, []int{0, 5, 12}, result.MatchedIndices)
}
