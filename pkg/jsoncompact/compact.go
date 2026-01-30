// Package jsoncompact provides JSON compression by trimming arrays to a configurable maximum.
package jsoncompact

import (
	"encoding/json"
	"fmt"
)

// Options controls JSON compaction behavior.
type Options struct {
	MaxArrayItems int // Trim arrays to N items (0 = no limit)
	MaxStringLen  int // Truncate strings longer than N chars (0 = no limit)
	MaxDepth      int // Max recursion depth (0 = unlimited)
}

// Default values for compaction options.
const (
	DefaultMaxArrayItems = 3
	DefaultMaxStringLen  = 500
	DefaultMaxDepth      = 0 // unlimited
)

// DefaultOptions returns the default compaction settings.
func DefaultOptions() *Options {
	return &Options{
		MaxArrayItems: DefaultMaxArrayItems,
		MaxStringLen:  DefaultMaxStringLen,
		MaxDepth:      DefaultMaxDepth,
	}
}

// Compact compresses JSON bytes by trimming arrays and strings.
// Returns error if input is not valid JSON.
// If opts is nil, DefaultOptions() is used.
func Compact(data []byte, opts *Options) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	compacted := compactRecursive(v, opts, 0)
	return json.Marshal(compacted)
}

// CompactValue compresses a parsed JSON value (any type from json.Unmarshal).
// If opts is nil, DefaultOptions() is used.
func CompactValue(v any, opts *Options) any {
	if opts == nil {
		opts = DefaultOptions()
	}
	return compactRecursive(v, opts, 0)
}

func compactRecursive(v any, opts *Options, depth int) any {
	if opts.MaxDepth > 0 && depth >= opts.MaxDepth {
		return "[max depth]"
	}

	switch val := v.(type) {
	case []any:
		return compactArray(val, opts, depth)
	case map[string]any:
		return compactObject(val, opts, depth)
	case string:
		return compactString(val, opts)
	default:
		return v
	}
}

func compactString(s string, opts *Options) string {
	if opts.MaxStringLen <= 0 || len(s) <= opts.MaxStringLen {
		return s
	}
	remaining := len(s) - opts.MaxStringLen
	return s[:opts.MaxStringLen] + fmt.Sprintf("... (%d more chars)", remaining)
}

func compactArray(arr []any, opts *Options, depth int) []any {
	if len(arr) == 0 {
		return arr
	}

	// If no limit or within limit, just recurse into elements
	if opts.MaxArrayItems <= 0 || len(arr) <= opts.MaxArrayItems {
		result := make([]any, len(arr))
		for i, item := range arr {
			result[i] = compactRecursive(item, opts, depth+1)
		}
		return result
	}

	// Trim array and add indicator
	result := make([]any, opts.MaxArrayItems+1)
	for i := 0; i < opts.MaxArrayItems; i++ {
		result[i] = compactRecursive(arr[i], opts, depth+1)
	}
	remaining := len(arr) - opts.MaxArrayItems
	result[opts.MaxArrayItems] = fmt.Sprintf("... (%d more items)", remaining)
	return result
}

func compactObject(obj map[string]any, opts *Options, depth int) map[string]any {
	result := make(map[string]any, len(obj))
	for k, v := range obj {
		result[k] = compactRecursive(v, opts, depth+1)
	}
	return result
}
