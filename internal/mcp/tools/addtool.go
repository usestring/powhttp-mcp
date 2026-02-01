package tools

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddTool registers a tool with the server and validates that the output type's
// zero value passes the SDK's inferred JSON schema. This catches nil-slice bugs
// at startup rather than at runtime.
//
// Panics if the zero value of Out fails schema validation.
func AddTool[In, Out any](srv *sdkmcp.Server, t *sdkmcp.Tool, h sdkmcp.ToolHandlerFor[In, Out]) {
	CheckOutputSchema[Out](t.Name)
	sdkmcp.AddTool(srv, t, h)
}

// CheckOutputSchema validates that the zero value of T passes the JSON schema
// the MCP SDK would infer from it. Call this at init/registration time to catch
// nil-slice-as-null issues before they surface at runtime.
//
// Go's json.Marshal serializes nil slices as null, but the SDK infers
// "type": "array" from the Go type, so null fails schema validation. Adding
// omitzero to slice fields or initializing them to empty slices fixes this.
//
// Panics if validation fails. No-ops for the untyped "any" output or if schema
// inference itself fails (the SDK will report those separately).
func CheckOutputSchema[T any](toolName string) {
	rt := reflect.TypeFor[T]()
	if rt == reflect.TypeFor[any]() {
		return
	}
	// Follow pointer like the SDK does.
	elem := rt
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}

	schema, err := jsonschema.ForType(elem, &jsonschema.ForOptions{})
	if err != nil {
		return // schema inference failed; SDK will report this in AddTool
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{})
	if err != nil {
		return // resolution failed; SDK will report this in AddTool
	}

	zero := reflect.Zero(elem).Interface()
	data, err := json.Marshal(zero)
	if err != nil {
		return
	}

	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return
	}

	if err := resolved.Validate(&v); err != nil {
		panic(fmt.Sprintf(
			"AddTool %q: zero value of output type %s fails schema validation: %v\n"+
				"  JSON: %s\n"+
				"  Fix: add `omitzero` to nil-defaulting slice fields, or initialize them to empty slices",
			toolName, elem, err, data,
		))
	}
}
