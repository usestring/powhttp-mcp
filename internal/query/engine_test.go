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
