// Package query provides JQ-based querying for HTTP response bodies.
package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
)

// Engine executes JQ queries against JSON data.
type Engine struct{}

// NewEngine creates a new query engine.
func NewEngine() *Engine {
	return &Engine{}
}

// QueryResult contains the results of a JQ query.
type QueryResult struct {
	Values         []any          `json:"values"`                     // Extracted values
	Errors         []string       `json:"errors,omitempty"`           // Per-item errors (e.g., type mismatch)
	RawCount       int            `json:"raw_count"`                  // Count before deduplication
	MatchedIndices []int          `json:"matched_indices,omitempty"`  // Indices of inputs that produced values
	LabelCounts    map[string]int `json:"label_counts,omitempty"`     // Value count per label
}

// Query executes a JQ expression against JSON data.
// Returns the extracted values and any errors encountered.
func (e *Engine) Query(data []byte, expression string, deduplicate bool, maxResults int) (*QueryResult, error) {
	// Parse and compile the JQ expression
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}

	// Parse the JSON data
	var input any
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("invalid JSON data: %w", err)
	}

	// Execute the query
	result := &QueryResult{
		Values: make([]any, 0),
		Errors: make([]string, 0),
	}

	seen := make(map[string]bool)
	iter := code.Run(input)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, isErr := v.(error); isErr {
			result.Errors = append(result.Errors, formatJQError("query", err))
			continue
		}

		// Skip nil values
		if v == nil {
			continue
		}

		result.RawCount++

		// Handle deduplication
		if deduplicate {
			key := valueKey(v)
			if seen[key] {
				continue
			}
			seen[key] = true
		}

		result.Values = append(result.Values, v)

		// Check max results
		if maxResults > 0 && len(result.Values) >= maxResults {
			break
		}
	}

	return result, nil
}

// QueryMultiple executes a JQ expression against multiple JSON data inputs.
// Combines results from all inputs, optionally deduplicating across all.
// Optional labels can be provided to identify each data input in error messages.
func (e *Engine) QueryMultiple(dataList [][]byte, expression string, deduplicate bool, maxResults int) (*QueryResult, error) {
	return e.QueryMultipleWithLabels(dataList, nil, expression, deduplicate, maxResults)
}

// QueryMultipleWithLabels executes a JQ expression against multiple JSON data inputs.
// Labels identify each input for better error messages (e.g., entry IDs).
func (e *Engine) QueryMultipleWithLabels(dataList [][]byte, labels []string, expression string, deduplicate bool, maxResults int) (*QueryResult, error) {
	// Parse and compile the JQ expression once
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}

	result := &QueryResult{
		Values:      make([]any, 0),
		Errors:      make([]string, 0),
		LabelCounts: make(map[string]int),
	}

	seen := make(map[string]bool)
	seenErrors := make(map[string]bool) // Deduplicate similar errors
	matchedSet := make(map[int]bool)    // Track which indices produced values

	for i, data := range dataList {
		if maxResults > 0 && len(result.Values) >= maxResults {
			break
		}

		// Get label for this input
		label := fmt.Sprintf("body[%d]", i)
		if labels != nil && i < len(labels) && labels[i] != "" {
			label = labels[i]
		}

		// Parse the JSON data
		var input any
		if err := json.Unmarshal(data, &input); err != nil {
			errMsg := fmt.Sprintf("%s: invalid JSON: %v", label, err)
			if !seenErrors[errMsg] {
				result.Errors = append(result.Errors, errMsg)
				seenErrors[errMsg] = true
			}
			continue
		}

		// Execute the query
		iter := code.Run(input)

		for {
			if maxResults > 0 && len(result.Values) >= maxResults {
				break
			}

			v, ok := iter.Next()
			if !ok {
				break
			}

			if err, isErr := v.(error); isErr {
				errMsg := formatJQError(label, err)
				if !seenErrors[errMsg] {
					result.Errors = append(result.Errors, errMsg)
					seenErrors[errMsg] = true
				}
				continue
			}

			// Skip nil values
			if v == nil {
				continue
			}

			result.RawCount++
			result.LabelCounts[label]++

			// Track that this index produced a value
			if !matchedSet[i] {
				matchedSet[i] = true
			}

			// Handle deduplication
			if deduplicate {
				key := valueKey(v)
				if seen[key] {
					continue
				}
				seen[key] = true
			}

			result.Values = append(result.Values, v)
		}
	}

	// Convert matched set to slice
	for idx := range matchedSet {
		result.MatchedIndices = append(result.MatchedIndices, idx)
	}

	return result, nil
}

// formatJQError creates a helpful error message for JQ execution errors.
// It adds contextual hints to help users fix common issues.
//
// Note: Runtime JQ errors (like "cannot iterate over: null") are plain errors
// without typed wrappers in gojq, so we use string matching for user-facing hints.
// This is intentional - we're decorating display messages, not making control flow decisions.
func formatJQError(label string, err error) string {
	// Check for typed errors first
	var haltErr *gojq.HaltError
	if errors.As(err, &haltErr) {
		if haltErr.Value() == nil {
			return fmt.Sprintf("%s: query halted", label)
		}
		return fmt.Sprintf("%s: query halted with: %v", label, haltErr.Value())
	}

	errStr := err.Error()

	// Add user-friendly hints for common runtime errors.
	// These are plain errors from gojq without typed wrappers, so string matching
	// is used to provide helpful context in the output message.
	var hint string
	switch {
	case strings.Contains(errStr, "cannot iterate over: null"):
		hint = " (the path may not exist in this response)"
	case strings.Contains(errStr, "cannot index") && strings.Contains(errStr, "with"):
		hint = " (field not found or wrong type)"
	case strings.Contains(errStr, "object") && strings.Contains(errStr, "cannot be iterated"):
		hint = " (expected array but got object, try removing '[]')"
	case strings.Contains(errStr, "array") && strings.Contains(errStr, "cannot be indexed"):
		hint = " (expected object but got array, try adding '[]')"
	}

	return fmt.Sprintf("%s: %s%s", label, errStr, hint)
}

// valueKey creates a string key for deduplication.
func valueKey(v any) string {
	switch val := v.(type) {
	case string:
		return "s:" + val
	case float64:
		return fmt.Sprintf("n:%v", val)
	case bool:
		return fmt.Sprintf("b:%v", val)
	case nil:
		return "null"
	default:
		// For complex types, marshal to JSON
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("?:%v", val)
		}
		return "j:" + string(b)
	}
}

// ValidateExpression checks if a JQ expression is valid without executing it.
func (e *Engine) ValidateExpression(expression string) error {
	query, err := gojq.Parse(expression)
	if err != nil {
		var parseErr *gojq.ParseError
		if errors.As(err, &parseErr) {
			return fmt.Errorf("invalid jq expression at position %d: %w", parseErr.Offset, err)
		}
		return fmt.Errorf("invalid jq expression: %w", err)
	}

	_, err = gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("failed to compile jq expression: %w", err)
	}

	return nil
}
