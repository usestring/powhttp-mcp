package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryForm(t *testing.T) {
	body := []byte("username=john&email=john%40example.com&role=admin&role=editor")

	t.Run("specific key", func(t *testing.T) {
		result, err := QueryForm(body, "email", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Equal(t, "john@example.com", result.Values[0])
	})

	t.Run("multi-value key", func(t *testing.T) {
		result, err := QueryForm(body, "role", 0)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
	})

	t.Run("missing key", func(t *testing.T) {
		result, err := QueryForm(body, "missing", 0)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
	})

	t.Run("all keys star", func(t *testing.T) {
		result, err := QueryForm(body, "*", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		m, ok := result.Values[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "john", m["username"])
		assert.Equal(t, "john@example.com", m["email"])
	})

	t.Run("all keys dot", func(t *testing.T) {
		result, err := QueryForm(body, ".", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
	})

	t.Run("url decoded values", func(t *testing.T) {
		result, err := QueryForm(body, "email", 0)
		require.NoError(t, err)
		assert.Equal(t, "john@example.com", result.Values[0])
	})
}
