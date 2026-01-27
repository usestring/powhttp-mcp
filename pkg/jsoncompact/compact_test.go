package jsoncompact

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompact_BasicArrayTrimming(t *testing.T) {
	input := `{"items": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]}`
	opts := &Options{MaxArrayItems: 3}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	items := parsed["items"].([]any)
	assert.Len(t, items, 4) // 3 items + indicator
	assert.Equal(t, float64(1), items[0])
	assert.Equal(t, float64(2), items[1])
	assert.Equal(t, float64(3), items[2])
	assert.Equal(t, "... (7 more items)", items[3])
}

func TestCompact_ArrayWithinLimit(t *testing.T) {
	input := `{"items": [1, 2, 3]}`
	opts := &Options{MaxArrayItems: 5}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	items := parsed["items"].([]any)
	assert.Len(t, items, 3)
	assert.Equal(t, float64(1), items[0])
	assert.Equal(t, float64(2), items[1])
	assert.Equal(t, float64(3), items[2])
}

func TestCompact_NestedArrays(t *testing.T) {
	input := `{
		"users": [
			{"name": "Alice", "tags": ["a", "b", "c", "d", "e"]},
			{"name": "Bob", "tags": ["x", "y", "z", "w"]},
			{"name": "Charlie", "tags": ["1", "2"]},
			{"name": "Dave", "tags": []},
			{"name": "Eve", "tags": ["only"]}
		]
	}`
	opts := &Options{MaxArrayItems: 3}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	users := parsed["users"].([]any)
	assert.Len(t, users, 4) // 3 users + indicator

	// First user's tags should be trimmed
	alice := users[0].(map[string]any)
	aliceTags := alice["tags"].([]any)
	assert.Len(t, aliceTags, 4) // 3 tags + indicator
	assert.Equal(t, "... (2 more items)", aliceTags[3])

	// Check the users indicator
	assert.Equal(t, "... (2 more items)", users[3])
}

func TestCompact_EmptyArray(t *testing.T) {
	input := `{"items": []}`
	opts := &Options{MaxArrayItems: 3}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	items := parsed["items"].([]any)
	assert.Len(t, items, 0)
}

func TestCompact_EmptyInput(t *testing.T) {
	result, err := Compact([]byte{}, nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCompact_NilOptions(t *testing.T) {
	input := `{"items": [1, 2, 3, 4, 5]}`

	result, err := Compact([]byte(input), nil)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	// Nil options uses DefaultOptions (MaxArrayItems=3)
	items := parsed["items"].([]any)
	assert.Len(t, items, 4) // 3 + indicator
}

func TestCompact_InvalidJSON(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"malformed", `{"items": [1, 2, 3`},
		{"html", `<html><body>Hello</body></html>`},
		{"xml", `<?xml version="1.0"?><root><item>1</item></root>`},
		{"plain text", `Hello, World!`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compact([]byte(tc.input), nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid JSON")
		})
	}
}

func TestCompact_MaxDepth(t *testing.T) {
	input := `{
		"level1": {
			"level2": {
				"level3": {
					"level4": [1, 2, 3]
				}
			}
		}
	}`
	opts := &Options{MaxArrayItems: 3, MaxDepth: 3}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	// level1 -> level2 -> level3 should be replaced
	level1 := parsed["level1"].(map[string]any)
	level2 := level1["level2"].(map[string]any)
	assert.Equal(t, "[max depth]", level2["level3"])
}

func TestCompact_DisabledArrayCompaction(t *testing.T) {
	input := `{"items": [1, 2, 3, 4, 5]}`
	opts := &Options{MaxArrayItems: 0}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	items := parsed["items"].([]any)
	assert.Len(t, items, 5) // No trimming
}

func TestCompactValue(t *testing.T) {
	input := map[string]any{
		"items": []any{1, 2, 3, 4, 5},
	}
	opts := &Options{MaxArrayItems: 2}

	result := CompactValue(input, opts)

	resultMap := result.(map[string]any)
	items := resultMap["items"].([]any)
	assert.Len(t, items, 3) // 2 + indicator
	assert.Equal(t, "... (3 more items)", items[2])
}

func TestCompact_PreservesNonArrayTypes(t *testing.T) {
	input := `{
		"string": "hello",
		"number": 42,
		"float": 3.14,
		"bool": true,
		"null": null,
		"nested": {"key": "value"}
	}`
	opts := &Options{MaxArrayItems: 3}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "hello", parsed["string"])
	assert.Equal(t, float64(42), parsed["number"])
	assert.Equal(t, 3.14, parsed["float"])
	assert.Equal(t, true, parsed["bool"])
	assert.Nil(t, parsed["null"])
	assert.Equal(t, "value", parsed["nested"].(map[string]any)["key"])
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, DefaultMaxArrayItems, opts.MaxArrayItems)
	assert.Equal(t, DefaultMaxStringLen, opts.MaxStringLen)
	assert.Equal(t, DefaultMaxDepth, opts.MaxDepth)
}

func TestCompact_StringTruncation(t *testing.T) {
	longString := "This is a very long string that should be truncated"
	input := `{"content": "` + longString + `"}`
	opts := &Options{MaxStringLen: 20}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	content := parsed["content"].(string)
	assert.Contains(t, content, "This is a very long ")
	assert.Contains(t, content, "... (")
	assert.Contains(t, content, " more chars)")
}

func TestCompact_StringWithinLimit(t *testing.T) {
	input := `{"content": "short"}`
	opts := &Options{MaxStringLen: 100}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "short", parsed["content"])
}

func TestCompact_StringTruncationDisabled(t *testing.T) {
	longString := "This is a very long string"
	input := `{"content": "` + longString + `"}`
	opts := &Options{MaxStringLen: 0}

	result, err := Compact([]byte(input), opts)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, longString, parsed["content"])
}
