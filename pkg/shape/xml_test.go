package shape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractXMLHierarchy_Basic(t *testing.T) {
	body := []byte(`<root><child>text</child></root>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	require.NotNil(t, h.Root)
	assert.Equal(t, "root", h.Root.Name)
	assert.Len(t, h.Root.Children, 1)
	assert.Equal(t, "child", h.Root.Children[0].Name)
}

func TestExtractXMLHierarchy_NestedElements(t *testing.T) {
	body := []byte(`<catalog>
		<book><title>Go</title><author>Rob</author></book>
		<book><title>Rust</title><author>Steve</author></book>
	</catalog>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	assert.Equal(t, "catalog", h.Root.Name)
	// "book" appears twice, should be marked as repeated
	require.Len(t, h.Root.Children, 1) // Deduplicated
	assert.Equal(t, "book", h.Root.Children[0].Name)
	assert.True(t, h.Root.Children[0].Repeated)
	assert.Len(t, h.Root.Children[0].Children, 2) // title, author
}

func TestExtractXMLHierarchy_Attributes(t *testing.T) {
	body := []byte(`<user id="123" role="admin"><name>Alice</name></user>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	assert.Equal(t, "user", h.Root.Name)
	assert.Contains(t, h.Root.Attributes, "id")
	assert.Contains(t, h.Root.Attributes, "role")
}

func TestExtractXMLHierarchy_RepeatedChildren(t *testing.T) {
	body := []byte(`<list>
		<item>1</item>
		<item>2</item>
		<item>3</item>
	</list>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	require.Len(t, h.Root.Children, 1)
	assert.True(t, h.Root.Children[0].Repeated)
	assert.Equal(t, "item", h.Root.Children[0].Name)
}

func TestExtractXMLHierarchy_Namespaces(t *testing.T) {
	body := []byte(`<root xmlns="http://example.com"><child>text</child></root>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	// Should strip namespace prefixes for single-namespace docs
	assert.Equal(t, "root", h.Root.Name)
}

func TestExtractXMLHierarchy_DepthLimit(t *testing.T) {
	body := []byte(`<l1><l2><l3><l4><l5><l6><l7>deep</l7></l6></l5></l4></l3></l2></l1>`)

	h, err := ExtractXMLHierarchy(body)
	require.NoError(t, err)
	assert.True(t, h.Truncated)
}

func TestExtractXMLHierarchy_EmptyInput(t *testing.T) {
	body := []byte(``)

	_, err := ExtractXMLHierarchy(body)
	assert.Error(t, err)
}

func TestMergeXMLHierarchy_DifferentChildren(t *testing.T) {
	body1 := []byte(`<root><name>Alice</name></root>`)
	body2 := []byte(`<root><name>Bob</name><age>30</age></root>`)

	h1, err := ExtractXMLHierarchy(body1)
	require.NoError(t, err)
	h2, err := ExtractXMLHierarchy(body2)
	require.NoError(t, err)

	mergeXMLHierarchy(h1, h2)

	require.NotNil(t, h1.Root)
	childNames := make(map[string]bool)
	for _, c := range h1.Root.Children {
		childNames[c.Name] = true
	}
	assert.True(t, childNames["name"], "merged hierarchy should contain 'name'")
	assert.True(t, childNames["age"], "merged hierarchy should contain 'age' from second sample")
}

func TestMergeXMLHierarchy_MergesAttributes(t *testing.T) {
	body1 := []byte(`<item id="1">text</item>`)
	body2 := []byte(`<item id="2" status="active">text</item>`)

	h1, err := ExtractXMLHierarchy(body1)
	require.NoError(t, err)
	h2, err := ExtractXMLHierarchy(body2)
	require.NoError(t, err)

	mergeXMLHierarchy(h1, h2)

	assert.Contains(t, h1.Root.Attributes, "id")
	assert.Contains(t, h1.Root.Attributes, "status")
}

func TestMergeXMLHierarchy_NilRoot(t *testing.T) {
	h1 := &XMLElementHierarchy{}
	body2 := []byte(`<root><child>text</child></root>`)

	h2, err := ExtractXMLHierarchy(body2)
	require.NoError(t, err)

	mergeXMLHierarchy(h1, h2)
	require.NotNil(t, h1.Root)
	assert.Equal(t, "root", h1.Root.Name)
}
