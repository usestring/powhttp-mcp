package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
