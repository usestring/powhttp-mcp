package textquery

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/usestring/powhttp-mcp/internal/query"
	"github.com/usestring/powhttp-mcp/pkg/contenttype"
)

// Engine dispatches text extraction queries to mode-specific handlers.
type Engine struct {
	jq *query.Engine
}

// NewEngine creates a new text query engine.
func NewEngine() *Engine {
	return &Engine{
		jq: query.NewEngine(),
	}
}

// Query extracts data from a body using the specified mode and expression.
// If mode is empty, it is auto-detected from the content type.
func (e *Engine) Query(body []byte, contentType, expression, mode string, maxResults int) (*QueryResult, error) {
	if mode == "" {
		mode = DetectMode(contentType)
	}

	switch mode {
	case ModeCSS:
		return QueryCSS(body, expression, maxResults)
	case ModeXPath:
		return QueryXPath(body, contentType, expression, maxResults)
	case ModeRegex:
		return QueryRegex(body, expression, maxResults)
	case ModeForm:
		return QueryForm(body, expression, maxResults)
	case ModeJQ:
		return e.queryJQ(body, contentType, expression, maxResults)
	default:
		return nil, fmt.Errorf("unknown mode: %q (valid: css, xpath, regex, form, jq)", mode)
	}
}

// ValidateExpression checks if an expression is valid for the given mode.
func (e *Engine) ValidateExpression(expression, mode string) error {
	switch mode {
	case ModeCSS:
		// goquery validates on use; basic check here
		if expression == "" {
			return fmt.Errorf("CSS selector expression is required")
		}
		return nil
	case ModeXPath:
		if expression == "" {
			return fmt.Errorf("XPath expression is required")
		}
		return nil
	case ModeRegex:
		_, err := QueryRegex(nil, expression, 0)
		if err != nil {
			return err
		}
		return nil
	case ModeForm:
		if expression == "" {
			return fmt.Errorf("form key expression is required")
		}
		return nil
	case ModeJQ:
		return e.jq.ValidateExpression(expression)
	default:
		return fmt.Errorf("unknown mode: %q", mode)
	}
}

// queryJQ applies a JQ expression to a body. For JSON content types the body
// is used directly; for YAML it is first converted to JSON.
func (e *Engine) queryJQ(body []byte, contentType, expression string, maxResults int) (*QueryResult, error) {
	jsonBytes := body

	// If content is not JSON, treat it as YAML and convert.
	if !contenttype.IsJSON(contentType) {
		var yamlData any
		if err := yaml.Unmarshal(body, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		var err error
		jsonBytes, err = json.Marshal(ConvertYAMLToJSON(yamlData))
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}
	}

	jqResult, err := e.jq.Query(jsonBytes, expression, false, maxResults)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Values: jqResult.Values,
		Count:  len(jqResult.Values),
		Mode:   ModeJQ,
		Errors: jqResult.Errors,
	}, nil
}

// ConvertYAMLToJSON recursively converts YAML-parsed values to JSON-compatible types.
// yaml.v3 produces map[string]any for mappings, but may produce other map types
// for non-string keys. Exported for reuse by pkg/shape.
func ConvertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = ConvertYAMLToJSON(v)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = ConvertYAMLToJSON(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = ConvertYAMLToJSON(v)
		}
		return result
	default:
		return v
	}
}
