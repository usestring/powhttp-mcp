package shape

import (
	"bytes"
	"encoding/xml"
	"strings"
)

const xmlMaxDepth = 5

// ExtractXMLHierarchy parses an XML body and returns a structural outline
// of the element tree with tag names, attributes, child counts, and
// repeated element flags. Limits depth to prevent excessive output.
func ExtractXMLHierarchy(body []byte) (*XMLElementHierarchy, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false

	hierarchy := &XMLElementHierarchy{
		MaxDepth: 0,
	}

	// Parse the root element
	root, maxDepth, truncated, err := parseXMLElement(decoder, 0, xmlMaxDepth)
	if err != nil {
		return nil, err
	}

	hierarchy.Root = root
	hierarchy.MaxDepth = maxDepth
	hierarchy.Truncated = truncated

	return hierarchy, nil
}

// parseXMLElement recursively parses XML elements from the decoder.
func parseXMLElement(decoder *xml.Decoder, depth, maxDepth int) (*XMLElement, int, bool, error) {
	truncated := false
	reachedDepth := depth

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, reachedDepth, truncated, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elem := &XMLElement{
				Name: stripNamespace(t.Name),
			}

			// Collect attributes
			for _, attr := range t.Attr {
				elem.Attributes = append(elem.Attributes, stripNamespace(attr.Name))
			}

			// Parse children
			if depth < maxDepth {
				children, childDepth, childTruncated := parseChildren(decoder, depth+1, maxDepth)
				elem.Children = children
				elem.ChildCount = len(children)
				if childTruncated {
					truncated = true
				}
				if childDepth > reachedDepth {
					reachedDepth = childDepth
				}
			} else {
				truncated = true
				// Skip the rest of this element
				decoder.Skip()
			}

			return elem, reachedDepth, truncated, nil

		case xml.EndElement:
			return nil, reachedDepth, truncated, nil
		}
	}
}

// parseChildren parses all child elements of the current element.
func parseChildren(decoder *xml.Decoder, depth, maxDepth int) ([]*XMLElement, int, bool) {
	truncated := false
	reachedDepth := depth
	childNames := make(map[string]int)
	var children []*XMLElement

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			name := stripNamespace(t.Name)
			childNames[name]++

			if depth > maxDepth {
				truncated = true
				decoder.Skip()
				continue
			}

			elem := &XMLElement{
				Name: name,
			}

			for _, attr := range t.Attr {
				elem.Attributes = append(elem.Attributes, stripNamespace(attr.Name))
			}

			if depth < maxDepth {
				subChildren, childDepth, childTruncated := parseChildren(decoder, depth+1, maxDepth)
				elem.Children = subChildren
				elem.ChildCount = len(subChildren)
				if childTruncated {
					truncated = true
				}
				if childDepth > reachedDepth {
					reachedDepth = childDepth
				}
			} else {
				truncated = true
				decoder.Skip()
			}

			// Only add the element once for repeated siblings
			if childNames[name] == 1 {
				children = append(children, elem)
			} else if childNames[name] == 2 {
				// Mark the first occurrence as repeated
				for _, c := range children {
					if c.Name == name {
						c.Repeated = true
						break
					}
				}
			}

		case xml.EndElement:
			return children, reachedDepth, truncated
		}
	}

	return children, reachedDepth, truncated
}

// mergeXMLHierarchy merges src into dst, adding any elements from src
// that don't already exist in dst and merging attributes.
func mergeXMLHierarchy(dst, src *XMLElementHierarchy) {
	if src.MaxDepth > dst.MaxDepth {
		dst.MaxDepth = src.MaxDepth
	}
	if src.Truncated {
		dst.Truncated = true
	}
	if dst.Root == nil {
		dst.Root = src.Root
		return
	}
	if src.Root != nil {
		mergeXMLElement(dst.Root, src.Root)
	}
}

// mergeXMLElement merges src element into dst, adding any children or
// attributes that appear in src but not dst.
func mergeXMLElement(dst, src *XMLElement) {
	// Merge attributes
	attrSet := make(map[string]bool)
	for _, a := range dst.Attributes {
		attrSet[a] = true
	}
	for _, a := range src.Attributes {
		if !attrSet[a] {
			dst.Attributes = append(dst.Attributes, a)
		}
	}

	// Merge children
	childIndex := make(map[string]*XMLElement)
	for _, c := range dst.Children {
		childIndex[c.Name] = c
	}
	for _, sc := range src.Children {
		if dc, exists := childIndex[sc.Name]; exists {
			// Recursively merge matching children
			mergeXMLElement(dc, sc)
			if sc.Repeated {
				dc.Repeated = true
			}
		} else {
			dst.Children = append(dst.Children, sc)
			dst.ChildCount = len(dst.Children)
		}
	}
}

// stripNamespace returns just the local part of an XML name,
// unless multiple namespaces are present.
func stripNamespace(name xml.Name) string {
	if name.Space != "" {
		// Only include namespace when it's not the default
		if !strings.HasPrefix(name.Space, "http://") && !strings.HasPrefix(name.Space, "https://") {
			return name.Space + ":" + name.Local
		}
	}
	return name.Local
}
