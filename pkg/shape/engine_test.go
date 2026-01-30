package shape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_AnalyzeJSON(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "name": "Bob"}`),
	}

	result, err := engine.Analyze(bodies, "application/json")
	require.NoError(t, err)
	assert.Equal(t, "json", result.ContentCategory)
	assert.NotNil(t, result.Schema)
	assert.NotNil(t, result.FieldStats)
	assert.Equal(t, 2, result.SampleCount)
}

func TestEngine_AnalyzeJSON_VendorType(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`{"status": "ok"}`),
	}

	result, err := engine.Analyze(bodies, "application/vnd.api+json")
	require.NoError(t, err)
	assert.Equal(t, "json", result.ContentCategory)
	assert.NotNil(t, result.Schema)
}

func TestEngine_AnalyzeYAML(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte("name: Alice\nage: 30\n"),
		[]byte("name: Bob\nage: 25\n"),
	}

	result, err := engine.Analyze(bodies, "application/yaml")
	require.NoError(t, err)
	assert.Equal(t, "yaml", result.ContentCategory)
	assert.NotNil(t, result.Schema)
	assert.Equal(t, 2, result.SampleCount)
}

func TestEngine_AnalyzeXML(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`<root><item id="1">Test</item></root>`),
	}

	result, err := engine.Analyze(bodies, "application/xml")
	require.NoError(t, err)
	assert.Equal(t, "xml", result.ContentCategory)
	assert.NotNil(t, result.XMLHierarchy)
}

func TestEngine_AnalyzeCSV(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte("name,age\nAlice,30\nBob,25\n"),
	}

	result, err := engine.Analyze(bodies, "text/csv")
	require.NoError(t, err)
	assert.Equal(t, "csv", result.ContentCategory)
	assert.NotNil(t, result.CSVColumns)
}

func TestEngine_AnalyzeHTML(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`),
	}

	result, err := engine.Analyze(bodies, "text/html")
	require.NoError(t, err)
	assert.Equal(t, "html", result.ContentCategory)
	assert.NotNil(t, result.HTMLOutline)
	assert.Equal(t, "Test", result.HTMLOutline.Title)
}

func TestEngine_AnalyzeForm(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte("username=alice&password=secret"),
		[]byte("username=bob&password=pass123"),
	}

	result, err := engine.Analyze(bodies, "application/x-www-form-urlencoded")
	require.NoError(t, err)
	assert.Equal(t, "form", result.ContentCategory)
	assert.Len(t, result.FormKeys, 2)
}

func TestEngine_AnalyzeBinary(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic bytes
	}

	result, err := engine.Analyze(bodies, "image/png")
	require.NoError(t, err)
	assert.Equal(t, "binary", result.ContentCategory)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "binary")
}

func TestEngine_AnalyzeEmpty(t *testing.T) {
	engine := NewEngine()
	_, err := engine.Analyze(nil, "application/json")
	assert.Error(t, err)
}

func TestEngine_AnalyzeXML_MultipleSamples(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`<root><name>Alice</name></root>`),
		[]byte(`<root><name>Bob</name><age>30</age></root>`),
	}

	result, err := engine.Analyze(bodies, "application/xml")
	require.NoError(t, err)
	assert.Equal(t, "xml", result.ContentCategory)
	assert.Equal(t, 2, result.SampleCount)

	childNames := make(map[string]bool)
	for _, c := range result.XMLHierarchy.Root.Children {
		childNames[c.Name] = true
	}
	assert.True(t, childNames["name"])
	assert.True(t, childNames["age"], "element from second sample should be merged")
}

func TestEngine_AnalyzeCSV_MultipleSamples(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte("name,age\nAlice,30\n"),
		[]byte("name,age\nBob,25\nCharlie,35\n"),
	}

	result, err := engine.Analyze(bodies, "text/csv")
	require.NoError(t, err)
	assert.Equal(t, "csv", result.ContentCategory)
	assert.Equal(t, 2, result.SampleCount)
	assert.Equal(t, 3, result.CSVColumns.RowCount)
}

func TestEngine_AnalyzeHTML_MultipleSamples(t *testing.T) {
	engine := NewEngine()
	bodies := [][]byte{
		[]byte(`<html><head><title>Page 1</title></head><body><div id="a">A</div></body></html>`),
		[]byte(`<html><head><title>Page 2</title></head><body><div id="b">B</div><span>C</span></body></html>`),
	}

	result, err := engine.Analyze(bodies, "text/html")
	require.NoError(t, err)
	assert.Equal(t, "html", result.ContentCategory)
	assert.Equal(t, 2, result.SampleCount)
	assert.Equal(t, "Page 1", result.HTMLOutline.Title)
	assert.Greater(t, result.HTMLOutline.TagCounts["span"], 0)

	ids := make(map[string]bool)
	for _, eid := range result.HTMLOutline.ElementIDs {
		ids[eid.ID] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["b"])
}

func TestEngine_FormKeys(t *testing.T) {
	bodies := [][]byte{
		[]byte("a=1&b=2"),
		[]byte("a=3&c=4"),
		[]byte("a=5&b=6&c=7"),
	}

	keys := extractFormKeys(bodies)
	assert.Len(t, keys, 3)

	byKey := make(map[string]FormKeyStat)
	for _, k := range keys {
		byKey[k.Key] = k
	}

	assert.Equal(t, 1.0, byKey["a"].Frequency)
	assert.InDelta(t, 0.666, byKey["b"].Frequency, 0.01)
	assert.InDelta(t, 0.666, byKey["c"].Frequency, 0.01)
}
