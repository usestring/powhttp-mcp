package tools

import (
	"encoding/json"
	"fmt"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestCheckOutputSchema_panicsOnNilSlice(t *testing.T) {
	type BadOutput struct {
		Items []string `json:"items"` // no omitzero → nil → null → schema expects array
	}
	assert.Panics(t, func() {
		CheckOutputSchema[BadOutput]("test_bad_tool")
	})
}

func TestCheckOutputSchema_okWithOmitzero(t *testing.T) {
	type GoodOutput struct {
		Items []string `json:"items,omitzero"`
	}
	assert.NotPanics(t, func() {
		CheckOutputSchema[GoodOutput]("test_good_tool")
	})
}

func TestCheckOutputSchema_okWithOmitempty(t *testing.T) {
	type GoodOutput struct {
		Items []string `json:"items,omitempty"`
	}
	assert.NotPanics(t, func() {
		CheckOutputSchema[GoodOutput]("test_good_tool")
	})
}

func TestCheckOutputSchema_okWithNoSlices(t *testing.T) {
	type SimpleOutput struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	assert.NotPanics(t, func() {
		CheckOutputSchema[SimpleOutput]("test_simple_tool")
	})
}

func TestCheckOutputSchema_okWithAny(t *testing.T) {
	assert.NotPanics(t, func() {
		CheckOutputSchema[any]("test_any_tool")
	})
}

func TestCheckOutputSchema_okWithPointerSlice(t *testing.T) {
	type PtrOutput struct {
		Items *[]string `json:"items"`
	}
	// Pointer to slice: zero value is nil pointer, serializes as null.
	// Schema allows null for pointer types, so this passes.
	assert.NotPanics(t, func() {
		CheckOutputSchema[PtrOutput]("test_ptr_tool")
	})
}

func TestCheckOutputSchema_panicsOnRawMessage(t *testing.T) {
	type BadOutput struct {
		Data json.RawMessage `json:"data,omitempty"`
	}
	assert.Panics(t, func() {
		CheckOutputSchema[BadOutput]("test_raw_message")
	})
}

func TestCheckOutputSchema_panicsOnRawMessageSlice(t *testing.T) {
	type BadOutput struct {
		Items []json.RawMessage `json:"items,omitzero"`
	}
	assert.Panics(t, func() {
		CheckOutputSchema[BadOutput]("test_raw_message_slice")
	})
}

func TestCheckOutputSchema_panicsOnNestedRawMessage(t *testing.T) {
	type Inner struct {
		Schema json.RawMessage `json:"schema,omitempty"`
	}
	type BadOutput struct {
		Nested Inner `json:"nested"`
	}
	assert.Panics(t, func() {
		CheckOutputSchema[BadOutput]("test_nested_raw_message")
	})
}

func TestCheckOutputSchema_nilSlicePanicShowsFieldPath(t *testing.T) {
	type Inner struct {
		Tags []string `json:"tags"` // nil slice, no omitzero
	}
	type BadOutput struct {
		Task Inner `json:"task"`
	}
	msg := recoverPanicString(func() {
		CheckOutputSchema[BadOutput]("test_field_path")
	})
	assert.Contains(t, msg, "Task.Tags")
	assert.Contains(t, msg, "[]string")
}

func TestCheckOutputSchema_nestedNilSlicePanicShowsFullPath(t *testing.T) {
	type Level2 struct {
		IDs []int `json:"ids"` // nil slice, no omitzero
	}
	type Level1 struct {
		Child Level2 `json:"child"`
	}
	type BadOutput struct {
		Parent Level1 `json:"parent"`
	}
	msg := recoverPanicString(func() {
		CheckOutputSchema[BadOutput]("test_deep_path")
	})
	assert.Contains(t, msg, "Parent.Child.IDs")
	assert.Contains(t, msg, "[]int")
}

// recoverPanicString calls f and returns the panic value as a string.
// Returns "" if f does not panic.
func recoverPanicString(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprintf("%v", r)
		}
	}()
	f()
	return ""
}

func TestCheckOutputSchema_panicsOnNilSliceInSliceElement(t *testing.T) {
	type Stat struct {
		Path     string `json:"path"`
		Examples []any  `json:"examples"` // nil → null at runtime
	}
	type Output struct {
		Stats []Stat `json:"stats,omitempty"`
	}
	msg := recoverPanicString(func() {
		CheckOutputSchema[Output]("test_slice_element_nil")
	})
	assert.Contains(t, msg, "Examples")
	assert.Contains(t, msg, "Stat")
}

func TestCheckOutputSchema_okWithAnySlice(t *testing.T) {
	type GoodOutput struct {
		Items []any `json:"items,omitzero"`
	}
	assert.NotPanics(t, func() {
		CheckOutputSchema[GoodOutput]("test_any_slice")
	})
}

func TestReplaceBoolSchemas_propertiesTrue(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"shape":   true,
			"summary": map[string]any{"type": "string"},
		},
	}
	changed := replaceBoolSchemas(m)
	assert.True(t, changed)
	props := m["properties"].(map[string]any)
	assert.Equal(t, map[string]any{}, props["shape"])
	assert.Equal(t, map[string]any{"type": "string"}, props["summary"])
}

func TestReplaceBoolSchemas_itemsTrue(t *testing.T) {
	m := map[string]any{
		"type":  "array",
		"items": true,
	}
	changed := replaceBoolSchemas(m)
	assert.True(t, changed)
	assert.Equal(t, map[string]any{}, m["items"])
}

func TestReplaceBoolSchemas_additionalPropertiesFalseUntouched(t *testing.T) {
	m := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
	changed := replaceBoolSchemas(m)
	assert.False(t, changed)
	assert.Equal(t, false, m["additionalProperties"])
}

func TestReplaceBoolSchemas_nested(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"inner": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"data": true,
				},
			},
		},
	}
	changed := replaceBoolSchemas(m)
	assert.True(t, changed)
	inner := m["properties"].(map[string]any)["inner"].(map[string]any)
	innerProps := inner["properties"].(map[string]any)
	assert.Equal(t, map[string]any{}, innerProps["data"])
}

func TestReplaceBoolSchemas_noChange(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	changed := replaceBoolSchemas(m)
	assert.False(t, changed)
}

func TestFixBooleanSchemas_setsOutputSchema(t *testing.T) {
	type WithAnyField struct {
		Shape   any    `json:"shape,omitzero"`
		Summary string `json:"summary"`
	}

	tool := &sdkmcp.Tool{Name: "test_tool"}
	fixBooleanSchemas[WithAnyField](tool)

	assert.NotNil(t, tool.OutputSchema)
	m, ok := tool.OutputSchema.(map[string]any)
	assert.True(t, ok)
	props := m["properties"].(map[string]any)
	// shape should be {} not true
	assert.Equal(t, map[string]any{}, props["shape"])
}

func TestFixBooleanSchemas_noop(t *testing.T) {
	type SimpleOutput struct {
		Name string `json:"name"`
	}

	tool := &sdkmcp.Tool{Name: "test_tool"}
	fixBooleanSchemas[SimpleOutput](tool)

	// No any fields, so OutputSchema should not be set
	assert.Nil(t, tool.OutputSchema)
}

func TestFixBooleanSchemas_noopForAny(t *testing.T) {
	tool := &sdkmcp.Tool{Name: "test_tool"}
	fixBooleanSchemas[any](tool)
	assert.Nil(t, tool.OutputSchema)
}
