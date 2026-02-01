package tools

import (
	"encoding/json"
	"testing"

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

func TestCheckOutputSchema_okWithAnySlice(t *testing.T) {
	type GoodOutput struct {
		Items []any `json:"items,omitzero"`
	}
	assert.NotPanics(t, func() {
		CheckOutputSchema[GoodOutput]("test_any_slice")
	})
}
