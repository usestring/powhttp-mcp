package indexer

import (
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
const tokenDelimiters = "/?&=.-_:"

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

	// Add query parameter keys (not values)
	for key := range parsed.Query() {
		parts = append(parts, key)
	}

	return Tokenize(strings.Join(parts, " "))
}

// TokenizePath extracts tokens from just the path portion.
func TokenizePath(path string) []string {
	return Tokenize(path)
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
