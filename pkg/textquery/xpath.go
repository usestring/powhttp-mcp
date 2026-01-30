package textquery

import (
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xmlquery"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
)

// QueryXPath extracts text content from XML or HTML using XPath expressions.
// Uses xmlquery for XML content types and htmlquery for HTML.
func QueryXPath(body []byte, ct, expression string, maxResults int) (*QueryResult, error) {
	category := contenttype.Classify(ct)

	if category == contenttype.HTML {
		return queryXPathHTML(body, expression, maxResults)
	}
	return queryXPathXML(body, expression, maxResults)
}

func queryXPathXML(body []byte, expression string, maxResults int) (*QueryResult, error) {
	doc, err := xmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	nodes, err := xmlquery.QueryAll(doc, expression)
	if err != nil {
		return nil, fmt.Errorf("invalid XPath expression: %w", err)
	}

	var values []any
	for _, node := range nodes {
		if maxResults > 0 && len(values) >= maxResults {
			break
		}
		text := strings.TrimSpace(node.InnerText())
		if text != "" {
			values = append(values, text)
		}
	}

	if values == nil {
		values = []any{}
	}

	return &QueryResult{
		Values: values,
		Count:  len(values),
		Mode:   ModeXPath,
	}, nil
}

func queryXPathHTML(body []byte, expression string, maxResults int) (*QueryResult, error) {
	doc, err := htmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	nodes, err := htmlquery.QueryAll(doc, expression)
	if err != nil {
		return nil, fmt.Errorf("invalid XPath expression: %w", err)
	}

	var values []any
	for _, node := range nodes {
		if maxResults > 0 && len(values) >= maxResults {
			break
		}
		text := strings.TrimSpace(htmlquery.InnerText(node))
		if text != "" {
			values = append(values, text)
		}
	}

	if values == nil {
		values = []any{}
	}

	return &QueryResult{
		Values: values,
		Count:  len(values),
		Mode:   ModeXPath,
	}, nil
}
