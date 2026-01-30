package textquery

import (
	"fmt"
	"regexp"
)

// QueryRegex extracts matches from text using Go regular expressions.
// When the regex has capture groups, returns the first capture group per match.
// When it has no capture groups, returns the full match.
func QueryRegex(body []byte, expression string, maxResults int) (*QueryResult, error) {
	re, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	hasGroups := re.NumSubexp() > 0
	matches := re.FindAllSubmatch(body, -1)

	var values []any
	for _, match := range matches {
		if maxResults > 0 && len(values) >= maxResults {
			break
		}
		if hasGroups && len(match) > 1 {
			values = append(values, string(match[1]))
		} else {
			values = append(values, string(match[0]))
		}
	}

	if values == nil {
		values = []any{}
	}

	return &QueryResult{
		Values: values,
		Count:  len(values),
		Mode:   ModeRegex,
	}, nil
}
