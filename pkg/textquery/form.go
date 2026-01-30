package textquery

import (
	"fmt"
	"net/url"
)

// QueryForm extracts values from form-urlencoded bodies by key name.
// Expression "*" or "." returns all key-value pairs as a map.
// A specific key name returns the values for that key.
func QueryForm(body []byte, expression string, maxResults int) (*QueryResult, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	var result []any

	if expression == "*" || expression == "." {
		// Return all key-value pairs as a map
		m := make(map[string]any)
		for key, vals := range values {
			if len(vals) == 1 {
				m[key] = vals[0]
			} else {
				anyVals := make([]any, len(vals))
				for i, v := range vals {
					anyVals[i] = v
				}
				m[key] = anyVals
			}
		}
		result = append(result, m)
	} else {
		// Return values for a specific key
		vals := values[expression]
		for _, v := range vals {
			if maxResults > 0 && len(result) >= maxResults {
				break
			}
			result = append(result, v)
		}
	}

	if result == nil {
		result = []any{}
	}

	return &QueryResult{
		Values: result,
		Count:  len(result),
		Mode:   ModeForm,
	}, nil
}
