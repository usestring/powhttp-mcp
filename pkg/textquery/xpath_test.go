package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryXPath_XML(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?>
	<catalog>
		<item><title>Book A</title><price>10</price></item>
		<item><title>Book B</title><price>20</price></item>
	</catalog>`)

	t.Run("extract titles", func(t *testing.T) {
		result, err := QueryXPath(xml, "application/xml", "//item/title", 0)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		assert.Equal(t, "Book A", result.Values[0])
		assert.Equal(t, "Book B", result.Values[1])
	})

	t.Run("max results", func(t *testing.T) {
		result, err := QueryXPath(xml, "application/xml", "//item/title", 1)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := QueryXPath(xml, "application/xml", "//missing", 0)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
	})

	t.Run("invalid xpath", func(t *testing.T) {
		_, err := QueryXPath(xml, "application/xml", "[invalid", 0)
		assert.Error(t, err)
	})
}

func TestQueryXPath_HTML(t *testing.T) {
	html := []byte(`<html><body>
		<h1>Title</h1>
		<p class="intro">Hello</p>
	</body></html>`)

	t.Run("xpath on html", func(t *testing.T) {
		result, err := QueryXPath(html, "text/html", "//h1", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Equal(t, "Title", result.Values[0])
	})
}
