package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryRegex(t *testing.T) {
	body := []byte(`status: active
status: pending
error: something went wrong
code: 404`)

	t.Run("with capture group", func(t *testing.T) {
		result, err := QueryRegex(body, `status:\s*(\w+)`, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		assert.Equal(t, "active", result.Values[0])
		assert.Equal(t, "pending", result.Values[1])
	})

	t.Run("without capture group", func(t *testing.T) {
		result, err := QueryRegex(body, `\d{3}`, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Equal(t, "404", result.Values[0])
	})

	t.Run("max results", func(t *testing.T) {
		result, err := QueryRegex(body, `status:\s*(\w+)`, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := QueryRegex(body, `zzz`, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
	})

	t.Run("invalid regex", func(t *testing.T) {
		_, err := QueryRegex(body, `[invalid`, 0)
		assert.Error(t, err)
	})

	t.Run("mode is regex", func(t *testing.T) {
		result, err := QueryRegex(body, `\w+`, 1)
		require.NoError(t, err)
		assert.Equal(t, ModeRegex, result.Mode)
	})
}
