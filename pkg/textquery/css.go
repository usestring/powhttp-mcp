package textquery

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// QueryCSS extracts text content from HTML using CSS selectors.
func QueryCSS(body []byte, expression string, maxResults int) (*QueryResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var values []any
	doc.Find(expression).Each(func(i int, s *goquery.Selection) {
		if maxResults > 0 && len(values) >= maxResults {
			return
		}
		text := strings.TrimSpace(s.Text())
		if text != "" {
			values = append(values, text)
		}
	})

	if values == nil {
		values = []any{}
	}

	return &QueryResult{
		Values: values,
		Count:  len(values),
		Mode:   ModeCSS,
	}, nil
}
