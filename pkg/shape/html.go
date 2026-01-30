package shape

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

const (
	htmlMaxElements = 200
	htmlMaxDepth    = 5
)

// ExtractHTMLOutline parses an HTML body and returns a structural summary
// including tag counts, elements with IDs, forms, and meta tags.
func ExtractHTMLOutline(body []byte) (*HTMLDOMOutline, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	outline := &HTMLDOMOutline{
		TagCounts:  make(map[string]int),
		ElementIDs: make([]HTMLElementID, 0),
		Forms:      make([]HTMLFormOutline, 0),
		MetaTags:   make([]HTMLMetaTag, 0),
	}

	elementCount := 0
	walkHTMLNode(doc, outline, &elementCount, 0)

	outline.Truncated = elementCount >= htmlMaxElements

	return outline, nil
}

// walkHTMLNode recursively processes HTML nodes to build the outline.
func walkHTMLNode(n *html.Node, outline *HTMLDOMOutline, elementCount *int, depth int) {
	if *elementCount >= htmlMaxElements || depth > htmlMaxDepth {
		return
	}

	if n.Type == html.ElementNode {
		*elementCount++

		tag := strings.ToLower(n.Data)
		outline.TagCounts[tag]++

		// Extract title
		if tag == "title" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
			outline.Title = strings.TrimSpace(n.FirstChild.Data)
		}

		// Track elements with IDs
		if id := getAttr(n, "id"); id != "" {
			outline.ElementIDs = append(outline.ElementIDs, HTMLElementID{
				Tag: tag,
				ID:  id,
			})
		}

		// Extract meta tags
		if tag == "meta" {
			name := getAttr(n, "name")
			if name == "" {
				name = getAttr(n, "property")
			}
			content := getAttr(n, "content")
			if name != "" || content != "" {
				outline.MetaTags = append(outline.MetaTags, HTMLMetaTag{
					Name:    name,
					Content: content,
				})
			}
		}

		// Extract forms
		if tag == "form" {
			form := HTMLFormOutline{
				Action: getAttr(n, "action"),
				Method: strings.ToUpper(getAttr(n, "method")),
				Inputs: collectFormInputs(n),
			}
			outline.Forms = append(outline.Forms, form)
		}
	}

	// Recurse into children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkHTMLNode(c, outline, elementCount, depth+1)
	}
}

// collectFormInputs extracts input elements from within a form.
func collectFormInputs(formNode *html.Node) []HTMLFormInput {
	var inputs []HTMLFormInput
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "input" || tag == "select" || tag == "textarea" {
				input := HTMLFormInput{
					Name: getAttr(n, "name"),
					Type: getAttr(n, "type"),
				}
				if tag == "select" {
					input.Type = "select"
				}
				if tag == "textarea" {
					input.Type = "textarea"
				}
				inputs = append(inputs, input)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := formNode.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return inputs
}

// mergeHTMLOutline merges src outline into dst, combining tag counts and
// adding any element IDs, forms, or meta tags that don't already exist.
func mergeHTMLOutline(dst, src *HTMLDOMOutline) {
	// Use the first non-empty title
	if dst.Title == "" && src.Title != "" {
		dst.Title = src.Title
	}

	// Sum tag counts
	for tag, count := range src.TagCounts {
		dst.TagCounts[tag] += count
	}

	// Merge element IDs (deduplicate by ID)
	idSet := make(map[string]bool)
	for _, eid := range dst.ElementIDs {
		idSet[eid.ID] = true
	}
	for _, eid := range src.ElementIDs {
		if !idSet[eid.ID] {
			dst.ElementIDs = append(dst.ElementIDs, eid)
			idSet[eid.ID] = true
		}
	}

	// Append forms (different pages may have different forms)
	dst.Forms = append(dst.Forms, src.Forms...)

	// Merge meta tags (deduplicate by name)
	metaSet := make(map[string]bool)
	for _, m := range dst.MetaTags {
		metaSet[m.Name] = true
	}
	for _, m := range src.MetaTags {
		if m.Name != "" && !metaSet[m.Name] {
			dst.MetaTags = append(dst.MetaTags, m)
			metaSet[m.Name] = true
		}
	}

	if src.Truncated {
		dst.Truncated = true
	}
}

// getAttr returns the value of a named attribute on a node, or empty string.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}
