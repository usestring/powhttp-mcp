package indexer

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Patterns for normalizing path segments
	uuidPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	numericPattern = regexp.MustCompile(`^\d+$`)
	hexPattern     = regexp.MustCompile(`^[0-9a-f]{8,}$`)
)

// tokenDelimiters defines characters that separate tokens
const tokenDelimiters = "/?&=.-_:;,@"

// Tokenize splits a string into searchable tokens.
// Splits on: / ? & = . - _ :
// Lowercases all tokens, drops tokens < 2 chars.
func Tokenize(s string) []string {
	s = strings.ToLower(s)

	// Split on delimiters
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(tokenDelimiters, r) || unicode.IsSpace(r)
	})

	// Filter tokens shorter than 2 characters
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len(t) >= 2 {
			result = append(result, t)
		}
	}

	return result
}

// NormalizePathSegment normalizes path segments for clustering.
// - numeric segments -> "{id}"
// - UUID patterns -> "{uuid}"
// - hex patterns (8+ chars) -> "{hex}"
func NormalizePathSegment(segment string) string {
	lower := strings.ToLower(segment)

	// Check UUID first (most specific)
	if uuidPattern.MatchString(lower) {
		return "{uuid}"
	}

	// Check pure numeric
	if numericPattern.MatchString(segment) {
		return "{id}"
	}

	// Check hex (8+ chars)
	if hexPattern.MatchString(lower) {
		return "{hex}"
	}

	return segment
}

// TokenizeURL extracts tokens from a full URL (host + path + query keys).
func TokenizeURL(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		// Fallback to basic tokenization
		return Tokenize(rawURL)
	}

	var parts []string

	// Add host tokens
	if parsed.Host != "" {
		parts = append(parts, parsed.Host)
	}

	// Add path tokens
	if parsed.Path != "" {
		parts = append(parts, parsed.Path)
	}

	// Add query parameter keys and values
	for key, values := range parsed.Query() {
		parts = append(parts, key)
		for _, v := range values {
			if v != "" {
				parts = append(parts, v)
			}
		}
	}

	return Tokenize(strings.Join(parts, " "))
}

// TokenizePath extracts tokens from just the path portion.
func TokenizePath(path string) []string {
	return Tokenize(path)
}

// TokenizeHeaders tokenizes full header fields ("name: value") using existing delimiter rules.
func TokenizeHeaders(headers []HeaderValue) []string {
	var parts []string
	for _, hv := range headers {
		parts = append(parts, hv.Name+": "+hv.Value)
	}
	if len(parts) == 0 {
		return nil
	}
	return Tokenize(strings.Join(parts, " "))
}

// TokenizeBody tokenizes body content based on content type.
// Handles JSON (keys + string values), HTML/XML (strip tags), plain text, and form-encoded.
// Skips binary content types. Processes up to maxBytes of decoded body.
func TokenizeBody(contentType string, bodyBytes []byte, maxBytes int) []string {
	if len(bodyBytes) == 0 {
		return nil
	}

	// Truncate to maxBytes
	if maxBytes > 0 && len(bodyBytes) > maxBytes {
		bodyBytes = bodyBytes[:maxBytes]
	}

	ct := strings.ToLower(contentType)

	switch {
	case strings.Contains(ct, "application/json"):
		return tokenizeJSON(bodyBytes)
	case strings.Contains(ct, "text/html"),
		strings.Contains(ct, "text/xml"),
		strings.Contains(ct, "application/xml"):
		return tokenizeStripTags(bodyBytes)
	case strings.Contains(ct, "text/plain"),
		strings.Contains(ct, "text/csv"):
		return Tokenize(string(bodyBytes))
	case strings.Contains(ct, "application/x-www-form-urlencoded"):
		return tokenizeFormEncoded(bodyBytes)
	default:
		// Skip binary and unknown content types
		return nil
	}
}

// tokenizeJSON extracts object keys and string values from JSON, then tokenizes them.
func tokenizeJSON(data []byte) []string {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		// Fallback: try tokenizing as plain text
		return Tokenize(string(data))
	}

	var parts []string
	extractJSONStrings(raw, &parts)
	if len(parts) == 0 {
		return nil
	}
	return Tokenize(strings.Join(parts, " "))
}

// extractJSONStrings recursively extracts object keys and string values from parsed JSON.
func extractJSONStrings(v any, out *[]string) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			*out = append(*out, k)
			extractJSONStrings(child, out)
		}
	case []any:
		for _, child := range val {
			extractJSONStrings(child, out)
		}
	case string:
		*out = append(*out, val)
	}
}

// tagStripper matches HTML/XML tags.
var tagStripper = regexp.MustCompile(`<[^>]*>`)

// tokenizeStripTags strips HTML/XML tags and tokenizes the visible text.
func tokenizeStripTags(data []byte) []string {
	text := tagStripper.ReplaceAllString(string(data), " ")
	return Tokenize(text)
}

// tokenizeFormEncoded parses form-encoded data and tokenizes keys and values.
func tokenizeFormEncoded(data []byte) []string {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return Tokenize(string(data))
	}

	var parts []string
	for key, vals := range values {
		parts = append(parts, key)
		for _, v := range vals {
			if v != "" {
				parts = append(parts, v)
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return Tokenize(strings.Join(parts, " "))
}

// NormalizePath normalizes a full path by normalizing each segment.
func NormalizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg != "" {
			segments[i] = NormalizePathSegment(seg)
		}
	}
	return strings.Join(segments, "/")
}
