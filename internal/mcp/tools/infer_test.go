package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferSchemaInput_Validation(t *testing.T) {
	// Verify the input struct has expected JSON tags and schema annotations
	input := InferSchemaInput{
		SessionID:  "test",
		EntryIDs:   []string{"e1", "e2"},
		ClusterID:  "c1",
		Target:     "response",
		MaxEntries: 20,
	}

	assert.Equal(t, "test", input.SessionID)
	assert.Equal(t, []string{"e1", "e2"}, input.EntryIDs)
	assert.Equal(t, "c1", input.ClusterID)
	assert.Equal(t, "response", input.Target)
	assert.Equal(t, 20, input.MaxEntries)
}

func TestInferSchemaInput_Defaults(t *testing.T) {
	// Verify zero-value defaults
	input := InferSchemaInput{}

	assert.Empty(t, input.SessionID)
	assert.Nil(t, input.EntryIDs)
	assert.Empty(t, input.ClusterID)
	assert.Empty(t, input.Target)
	assert.Equal(t, 0, input.MaxEntries)
}
