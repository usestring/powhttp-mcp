package tools

import (
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
