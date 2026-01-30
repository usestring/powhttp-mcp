package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryCSS(t *testing.T) {
	html := []byte(`<html><body>
		<h1 class="title">Hello</h1>
		<h1 class="title">World</h1>
		<div class="content"><span>Nested</span> text</div>
	</body></html>`)

	t.Run("single match", func(t *testing.T) {
		result, err := QueryCSS(html, "div.content", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Equal(t, "Nested text", result.Values[0])
	})

	t.Run("multiple matches", func(t *testing.T) {
		result, err := QueryCSS(html, "h1.title", 0)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		assert.Equal(t, "Hello", result.Values[0])
		assert.Equal(t, "World", result.Values[1])
	})

	t.Run("nested text", func(t *testing.T) {
		result, err := QueryCSS(html, "div.content", 0)
		require.NoError(t, err)
		assert.Contains(t, result.Values[0], "Nested")
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := QueryCSS(html, "h2.missing", 0)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
		assert.Empty(t, result.Values)
	})

	t.Run("max results", func(t *testing.T) {
		result, err := QueryCSS(html, "h1.title", 1)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
	})

	t.Run("mode is css", func(t *testing.T) {
		result, err := QueryCSS(html, "h1", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeCSS, result.Mode)
	})
}
