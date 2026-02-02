package graphql

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

// graphqlBody represents the JSON structure of a GraphQL request body.
type graphqlBody struct {
	Query         string `json:"query"`
	OperationName string `json:"operationName"`
	Variables     any    `json:"variables"`
}

// ParseRequestBody parses a GraphQL request body (single or batched).
// Returns the parsed operations. For non-JSON or empty bodies, returns an error.
func ParseRequestBody(body []byte) (*ParseResult, error) {
	body = bytes_trimSpace(body)
	if len(body) == 0 {
		return nil, newParseError(ErrEmpty, "graphql: empty body", nil)
	}

	// Try batched (JSON array) first
	if body[0] == '[' {
		var arr []graphqlBody
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, newParseError(nil, "graphql: invalid JSON array", err)
		}
		if len(arr) == 0 {
			return nil, newParseError(ErrEmpty, "graphql: empty batch array", nil)
		}
		ops := make([]ParsedOperation, 0, len(arr))
		for i, item := range arr {
			op := parseOne(item)
			op.BatchIndex = i
			ops = append(ops, op)
		}
		return &ParseResult{Operations: ops, IsBatched: true}, nil
	}

	// Single operation
	var single graphqlBody
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, newParseError(nil, "graphql: invalid JSON object", err)
	}
	if single.Query == "" && single.OperationName == "" {
		return nil, newParseError(ErrNotGraphQL, "graphql: not a GraphQL request body", nil)
	}

	op := parseOne(single)
	return &ParseResult{Operations: []ParsedOperation{op}}, nil
}

// parseOne converts a single graphqlBody into a ParsedOperation.
func parseOne(b graphqlBody) ParsedOperation {
	op := ParsedOperation{
		RawQuery:      b.Query,
		Variables:     b.Variables,
		HasVariables:  b.Variables != nil,
		OperationName: b.OperationName,
	}

	// Try to parse the query string
	opType, opName, fields, ok := scanQuery(b.Query)
	if ok {
		op.Type = opType
		op.Name = opName
		op.Fields = fields
	} else {
		op.ParseFailed = b.Query != "" // only mark failed if there was something to parse
		op.Type = "query"              // default type
	}

	// Prefer operationName from JSON body if present
	if b.OperationName != "" {
		op.Name = b.OperationName
	}

	// Default unnamed operations to "anonymous"
	if op.Name == "" {
		op.Name = "anonymous"
	}

	return op
}

// scanQuery extracts the operation type, name, and top-level field names
// from a GraphQL query string using simple string scanning.
// Returns (type, name, fields, ok).
func scanQuery(query string) (string, string, []string, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", "", nil, false
	}

	// Determine operation type by leading keyword
	opType := "query"
	rest := query

	for _, keyword := range []string{"subscription", "mutation", "query"} {
		if strings.HasPrefix(strings.ToLower(rest), keyword) {
			opType = keyword
			rest = strings.TrimSpace(rest[len(keyword):])
			break
		}
	}

	// If the query starts with '{', it's a shorthand query with no name
	if strings.HasPrefix(query, "{") {
		fields := extractTopLevelFields(query)
		return "query", "", fields, true
	}

	// Extract operation name (the identifier before '(' or '{')
	name := ""
	i := 0
	// Skip whitespace
	for i < len(rest) && unicode.IsSpace(rune(rest[i])) {
		i++
	}
	// Read identifier
	start := i
	for i < len(rest) && (unicode.IsLetter(rune(rest[i])) || unicode.IsDigit(rune(rest[i])) || rest[i] == '_') {
		i++
	}
	if i > start {
		name = rest[start:i]
	}

	// Extract top-level fields from the first selection set
	fields := extractTopLevelFields(rest)

	return opType, name, fields, true
}

// extractTopLevelFields finds the first '{...}' block and extracts
// the top-level field names (identifiers at brace depth 1).
// Skips content inside parentheses (arguments) and type names after
// inline fragment spreads (... on TypeName).
func extractTopLevelFields(s string) []string {
	// Find the first opening brace
	braceStart := strings.IndexByte(s, '{')
	if braceStart < 0 {
		return nil
	}

	var fields []string
	seen := make(map[string]bool)
	braceDepth := 0
	parenDepth := 0
	i := braceStart

	for i < len(s) {
		ch := s[i]
		switch ch {
		case '{':
			braceDepth++
			i++
		case '}':
			braceDepth--
			if braceDepth == 0 {
				return fields
			}
			i++
		case '(':
			parenDepth++
			i++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			i++
		case '#':
			// Skip line comments
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case '@':
			// Skip directive name (e.g., @include, @skip)
			i++
			for i < len(s) && isIdentChar(s[i]) {
				i++
			}
		case '.':
			// Handle spread operator (... or ...on)
			// Skip "..." and any following "on TypeName"
			if i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.' {
				i += 3
				// Skip whitespace
				for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
					i++
				}
				// Check for "on" keyword (inline fragment)
				if i+2 < len(s) && s[i] == 'o' && s[i+1] == 'n' && !isIdentChar(s[i+2]) {
					i += 2
					// Skip whitespace
					for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
						i++
					}
					// Skip the type name
					for i < len(s) && isIdentChar(s[i]) {
						i++
					}
				} else {
					// Named fragment spread: ...FragmentName — skip the name
					for i < len(s) && isIdentChar(s[i]) {
						i++
					}
				}
			} else {
				i++
			}
		default:
			if braceDepth == 1 && parenDepth == 0 && (unicode.IsLetter(rune(ch)) || ch == '_') {
				// Read identifier
				start := i
				for i < len(s) && isIdentChar(s[i]) {
					i++
				}
				fieldName := s[start:i]
				// Skip GraphQL keywords that appear at field level
				if !isGraphQLKeyword(fieldName) && !seen[fieldName] {
					fields = append(fields, fieldName)
					seen[fieldName] = true
				}
			} else {
				i++
			}
		}
	}

	return fields
}

// ExtractFragments scans a GraphQL query string for named fragments
// (fragment Foo on Bar { ... }) and inline fragments (... on Bar { ... }),
// returning a list of FragmentInfo.
func ExtractFragments(query string) []FragmentInfo {
	var fragments []FragmentInfo
	i := 0

	for i < len(query) {
		// Skip string literals
		if query[i] == '"' {
			i++
			for i < len(query) && query[i] != '"' {
				if query[i] == '\\' {
					i++ // skip escaped char
				}
				i++
			}
			if i < len(query) {
				i++ // closing quote
			}
			continue
		}

		// Skip line comments
		if query[i] == '#' {
			for i < len(query) && query[i] != '\n' {
				i++
			}
			continue
		}

		// Check for named fragment: "fragment Name on Type { ... }"
		if i+8 < len(query) && query[i:i+8] == "fragment" && (i == 0 || !isIdentChar(query[i-1])) && !isIdentChar(query[i+8]) {
			i += 8
			i = skipWS(query, i)
			// Read fragment name
			name, end := readIdent(query, i)
			if name == "" || name == "on" {
				continue
			}
			i = skipWS(query, end)
			// Expect "on"
			if i+2 <= len(query) && query[i:i+2] == "on" && (i+2 >= len(query) || !isIdentChar(query[i+2])) {
				i = skipWS(query, i+2)
				typeName, end := readIdent(query, i)
				if typeName != "" {
					i = end
					bodyStart := i
					fields := extractSelectionSetFields(query, &i)
					fragments = append(fragments, FragmentInfo{
						Name:   name,
						OnType: typeName,
						Fields: fields,
					})
					if i > bodyStart {
						nested := ExtractFragments(query[bodyStart:i])
						fragments = append(fragments, nested...)
					}
				}
			}
			continue
		}

		// Check for inline fragment: "... on Type { ... }"
		if i+3 <= len(query) && query[i:i+3] == "..." {
			j := skipWS(query, i+3)
			if j+2 <= len(query) && query[j:j+2] == "on" && (j+2 >= len(query) || !isIdentChar(query[j+2])) {
				j = skipWS(query, j+2)
				typeName, end := readIdent(query, j)
				if typeName != "" {
					j = end
					bodyStart := j
					fields := extractSelectionSetFields(query, &j)
					fragments = append(fragments, FragmentInfo{
						OnType:   typeName,
						IsInline: true,
						Fields:   fields,
					})
					if j > bodyStart {
						nested := ExtractFragments(query[bodyStart:j])
						fragments = append(fragments, nested...)
					}
					i = j
					continue
				}
			}
		}

		i++
	}

	return fragments
}

// skipWS advances past whitespace and returns the new position.
func skipWS(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == ',') {
		i++
	}
	return i
}

// readIdent reads a GraphQL identifier starting at position i.
// Returns the identifier and the position after it.
func readIdent(s string, i int) (string, int) {
	start := i
	for i < len(s) && isIdentChar(s[i]) {
		i++
	}
	if i == start {
		return "", i
	}
	return s[start:i], i
}

// extractSelectionSetFields reads the next selection set { ... } and returns
// top-level field names. Advances *pos past the closing brace.
func extractSelectionSetFields(s string, pos *int) []string {
	i := skipWS(s, *pos)
	if i >= len(s) || s[i] != '{' {
		*pos = i
		return nil
	}

	// Use the existing field extraction logic
	fields := extractTopLevelFields(s[i:])
	// Advance past the selection set
	depth := 0
	for i < len(s) {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				*pos = i + 1
				return fields
			}
		}
		i++
	}
	*pos = i
	return fields
}

// isIdentChar returns true for characters valid in a GraphQL identifier.
func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// isGraphQLKeyword returns true for GraphQL keywords that should not
// be treated as field names.
func isGraphQLKeyword(s string) bool {
	switch strings.ToLower(s) {
	case "fragment", "on", "true", "false", "null":
		return true
	}
	return false
}

// bytes_trimSpace trims whitespace from both ends of a byte slice.
func bytes_trimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && b[start] <= ' ' {
		start++
	}
	end := len(b)
	for end > start && b[end-1] <= ' ' {
		end--
	}
	return b[start:end]
}

// Sentinel errors for parse failures.
var (
	// ErrEmpty indicates the request body was empty or whitespace-only.
	ErrEmpty = errors.New("graphql: empty body")

	// ErrNotGraphQL indicates the body is valid JSON but does not contain
	// a GraphQL query or operationName field.
	ErrNotGraphQL = errors.New("graphql: not a GraphQL request body")
)

// ParseError wraps a parse failure with the underlying cause.
// Use errors.Is to check for ErrEmpty or ErrNotGraphQL, and
// errors.As to extract the ParseError for context.
type ParseError struct {
	// Sentinel is the category (ErrEmpty, ErrNotGraphQL, or nil for JSON errors).
	Sentinel error
	// Cause is the underlying error (e.g., json.SyntaxError).
	Cause error
	// Message provides human-readable context.
	Message string
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ParseError) Unwrap() error {
	if e.Sentinel != nil {
		return e.Sentinel
	}
	return e.Cause
}

// Is supports errors.Is matching against sentinel errors.
func (e *ParseError) Is(target error) bool {
	return e.Sentinel == target
}

// IsNotGraphQL returns true if the error indicates the body is not GraphQL.
// Checks for both ErrNotGraphQL and ErrEmpty using errors.Is.
func IsNotGraphQL(err error) bool {
	return errors.Is(err, ErrNotGraphQL) || errors.Is(err, ErrEmpty)
}

func newParseError(sentinel error, message string, cause error) *ParseError {
	return &ParseError{
		Sentinel: sentinel,
		Cause:    cause,
		Message:  message,
	}
}

// IsGraphQLBody probes whether a JSON body is a GraphQL request by checking
// for the presence of a "query" field (string) or an array of objects with
// "query" fields. This is more reliable than path-based detection because
// GraphQL endpoints can be mounted on any path.
//
// Returns true if the body looks like a GraphQL request, false otherwise.
// Does not fully parse the body — only checks structural signals.
func IsGraphQLBody(body []byte) bool {
	body = bytes_trimSpace(body)
	if len(body) == 0 {
		return false
	}

	if body[0] == '[' {
		// Batched: check first element
		var arr []json.RawMessage
		if err := json.Unmarshal(body, &arr); err != nil || len(arr) == 0 {
			return false
		}
		return hasQueryField(arr[0])
	}

	return hasQueryField(body)
}

// hasQueryField checks if a JSON object contains a non-empty "query" string field.
func hasQueryField(data []byte) bool {
	var obj struct {
		Query *string `json:"query"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	return obj.Query != nil && *obj.Query != ""
}
