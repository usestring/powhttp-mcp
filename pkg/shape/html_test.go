package shape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHTMLOutline_Basic(t *testing.T) {
	body := []byte(`<html>
		<head><title>Test Page</title></head>
		<body><h1>Hello</h1><p>World</p></body>
	</html>`)

	outline, err := ExtractHTMLOutline(body)
	require.NoError(t, err)
	assert.Equal(t, "Test Page", outline.Title)
	assert.Greater(t, outline.TagCounts["h1"], 0)
	assert.Greater(t, outline.TagCounts["p"], 0)
}

func TestExtractHTMLOutline_Forms(t *testing.T) {
	body := []byte(`<html><body>
		<form action="/login" method="POST">
			<input name="username" type="text"/>
			<input name="password" type="password"/>
			<select name="role"><option>admin</option></select>
			<textarea name="bio"></textarea>
			<button type="submit">Login</button>
		</form>
	</body></html>`)

	outline, err := ExtractHTMLOutline(body)
	require.NoError(t, err)
	require.Len(t, outline.Forms, 1)

	form := outline.Forms[0]
	assert.Equal(t, "/login", form.Action)
	assert.Equal(t, "POST", form.Method)
	assert.Len(t, form.Inputs, 4) // 2 inputs + select + textarea
}

func TestExtractHTMLOutline_ElementIDs(t *testing.T) {
	body := []byte(`<html><body>
		<div id="header">Header</div>
		<div id="content">Content</div>
		<div id="footer">Footer</div>
	</body></html>`)

	outline, err := ExtractHTMLOutline(body)
	require.NoError(t, err)
	assert.Len(t, outline.ElementIDs, 3)

	ids := make(map[string]string)
	for _, eid := range outline.ElementIDs {
		ids[eid.ID] = eid.Tag
	}
	assert.Equal(t, "div", ids["header"])
	assert.Equal(t, "div", ids["content"])
	assert.Equal(t, "div", ids["footer"])
}

func TestExtractHTMLOutline_MetaTags(t *testing.T) {
	body := []byte(`<html><head>
		<meta name="description" content="A test page"/>
		<meta property="og:title" content="Test"/>
	</head><body></body></html>`)

	outline, err := ExtractHTMLOutline(body)
	require.NoError(t, err)
	assert.Len(t, outline.MetaTags, 2)
	assert.Equal(t, "description", outline.MetaTags[0].Name)
	assert.Equal(t, "A test page", outline.MetaTags[0].Content)
}

func TestExtractHTMLOutline_LargePage(t *testing.T) {
	// Create a page with many elements
	body := []byte(`<html><body>`)
	for i := 0; i < 300; i++ {
		body = append(body, []byte(`<div><span>item</span></div>`)...)
	}
	body = append(body, []byte(`</body></html>`)...)

	outline, err := ExtractHTMLOutline(body)
	require.NoError(t, err)
	assert.True(t, outline.Truncated)
}

func TestMergeHTMLOutline_CombinesTagCounts(t *testing.T) {
	body1 := []byte(`<html><body><div>A</div><div>B</div></body></html>`)
	body2 := []byte(`<html><body><div>C</div><span>D</span></body></html>`)

	o1, err := ExtractHTMLOutline(body1)
	require.NoError(t, err)
	o2, err := ExtractHTMLOutline(body2)
	require.NoError(t, err)

	mergeHTMLOutline(o1, o2)

	assert.Equal(t, 3, o1.TagCounts["div"]) // 2 from body1 + 1 from body2
	assert.Equal(t, 1, o1.TagCounts["span"])
}

func TestMergeHTMLOutline_DeduplicatesIDs(t *testing.T) {
	body1 := []byte(`<html><body><div id="header">H</div></body></html>`)
	body2 := []byte(`<html><body><div id="header">H</div><div id="footer">F</div></body></html>`)

	o1, err := ExtractHTMLOutline(body1)
	require.NoError(t, err)
	o2, err := ExtractHTMLOutline(body2)
	require.NoError(t, err)

	mergeHTMLOutline(o1, o2)

	ids := make(map[string]bool)
	for _, eid := range o1.ElementIDs {
		ids[eid.ID] = true
	}
	assert.True(t, ids["header"])
	assert.True(t, ids["footer"])
	assert.Len(t, o1.ElementIDs, 2, "header should not be duplicated")
}

func TestMergeHTMLOutline_DeduplicatesMetaTags(t *testing.T) {
	body1 := []byte(`<html><head><meta name="description" content="Page 1"/></head><body></body></html>`)
	body2 := []byte(`<html><head><meta name="description" content="Page 2"/><meta name="author" content="Test"/></head><body></body></html>`)

	o1, err := ExtractHTMLOutline(body1)
	require.NoError(t, err)
	o2, err := ExtractHTMLOutline(body2)
	require.NoError(t, err)

	mergeHTMLOutline(o1, o2)

	names := make(map[string]bool)
	for _, m := range o1.MetaTags {
		names[m.Name] = true
	}
	assert.True(t, names["description"])
	assert.True(t, names["author"])
	assert.Len(t, o1.MetaTags, 2, "description should not be duplicated")
}

func TestMergeHTMLOutline_AppendsForms(t *testing.T) {
	body1 := []byte(`<html><body><form action="/login" method="POST"><input name="user" type="text"/></form></body></html>`)
	body2 := []byte(`<html><body><form action="/signup" method="POST"><input name="email" type="email"/></form></body></html>`)

	o1, err := ExtractHTMLOutline(body1)
	require.NoError(t, err)
	o2, err := ExtractHTMLOutline(body2)
	require.NoError(t, err)

	mergeHTMLOutline(o1, o2)

	assert.Len(t, o1.Forms, 2)
	assert.Equal(t, "/login", o1.Forms[0].Action)
	assert.Equal(t, "/signup", o1.Forms[1].Action)
}

func TestMergeHTMLOutline_UsesFirstTitle(t *testing.T) {
	body1 := []byte(`<html><head><title>Page One</title></head><body></body></html>`)
	body2 := []byte(`<html><head><title>Page Two</title></head><body></body></html>`)

	o1, err := ExtractHTMLOutline(body1)
	require.NoError(t, err)
	o2, err := ExtractHTMLOutline(body2)
	require.NoError(t, err)

	mergeHTMLOutline(o1, o2)

	assert.Equal(t, "Page One", o1.Title, "should keep the first non-empty title")
}
