package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Query(t *testing.T) {
	e := NewEngine()

	t.Run("json dispatch", func(t *testing.T) {
		jsonBody := []byte(`{"name":"Alice","age":30}`)
		result, err := e.Query(jsonBody, "application/json", ".name", "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeJQ, result.Mode)
		assert.Equal(t, "Alice", result.Values[0])
	})

	t.Run("json explicit jq mode", func(t *testing.T) {
		jsonBody := []byte(`[{"id":1},{"id":2}]`)
		result, err := e.Query(jsonBody, "application/json", ".[].id", ModeJQ, 0)
		require.NoError(t, err)
		assert.Equal(t, ModeJQ, result.Mode)
		assert.Len(t, result.Values, 2)
	})

	t.Run("css dispatch", func(t *testing.T) {
		html := []byte(`<html><body><h1>Hello</h1></body></html>`)
		result, err := e.Query(html, "text/html", "h1", "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeCSS, result.Mode)
		assert.Equal(t, "Hello", result.Values[0])
	})

	t.Run("xpath dispatch", func(t *testing.T) {
		xml := []byte(`<root><item>A</item></root>`)
		result, err := e.Query(xml, "application/xml", "//item", "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeXPath, result.Mode)
		assert.Equal(t, "A", result.Values[0])
	})

	t.Run("regex dispatch", func(t *testing.T) {
		text := []byte(`code: 200`)
		result, err := e.Query(text, "text/plain", `\d+`, "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeRegex, result.Mode)
		assert.Equal(t, "200", result.Values[0])
	})

	t.Run("form dispatch", func(t *testing.T) {
		form := []byte(`a=1&b=2`)
		result, err := e.Query(form, "application/x-www-form-urlencoded", "a", "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeForm, result.Mode)
		assert.Equal(t, "1", result.Values[0])
	})

	t.Run("yaml to jq", func(t *testing.T) {
		yaml := []byte("config:\n  timeout: 30\n  retries: 3\n")
		result, err := e.Query(yaml, "application/yaml", ".config.timeout", "", 0)
		require.NoError(t, err)
		assert.Equal(t, ModeJQ, result.Mode)
		require.Len(t, result.Values, 1)
		// yaml.v3 parses integers; JQ returns them as float64 via JSON roundtrip
		assert.EqualValues(t, 30, result.Values[0])
	})

	t.Run("explicit mode override", func(t *testing.T) {
		html := []byte(`<html><body><h1>Title</h1></body></html>`)
		result, err := e.Query(html, "text/html", "//h1", ModeXPath, 0)
		require.NoError(t, err)
		assert.Equal(t, ModeXPath, result.Mode)
		assert.Equal(t, "Title", result.Values[0])
	})

	t.Run("unknown mode error", func(t *testing.T) {
		_, err := e.Query([]byte("test"), "", "expr", "invalid", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown mode")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		badYAML := []byte(":\n  - [invalid\n  yaml: {")
		_, err := e.Query(badYAML, "application/yaml", ".foo", "", 0)
		assert.Error(t, err)
	})
}

func TestEngine_ValidateExpression(t *testing.T) {
	e := NewEngine()

	t.Run("valid css", func(t *testing.T) {
		assert.NoError(t, e.ValidateExpression("h1.title", ModeCSS))
	})

	t.Run("empty css", func(t *testing.T) {
		assert.Error(t, e.ValidateExpression("", ModeCSS))
	})

	t.Run("valid regex", func(t *testing.T) {
		assert.NoError(t, e.ValidateExpression(`\d+`, ModeRegex))
	})

	t.Run("invalid regex", func(t *testing.T) {
		assert.Error(t, e.ValidateExpression(`[invalid`, ModeRegex))
	})

	t.Run("valid jq", func(t *testing.T) {
		assert.NoError(t, e.ValidateExpression(".data.items[]", ModeJQ))
	})

	t.Run("unknown mode", func(t *testing.T) {
		assert.Error(t, e.ValidateExpression("test", "invalid"))
	})
}
