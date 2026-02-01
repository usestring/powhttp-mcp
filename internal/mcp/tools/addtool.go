package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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
// Also detects json.RawMessage fields, which serialize as transparent JSON but
// are inferred as []byte (array of integers) by the schema generator.
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

	// Check for json.RawMessage fields that would cause schema/runtime mismatch.
	if paths := findRawMessageFields(elem, nil, make(map[reflect.Type]bool)); len(paths) > 0 {
		panic(fmt.Sprintf(
			"AddTool %q: output type %s contains json.RawMessage at %s\n"+
				"  json.RawMessage serializes as transparent JSON but schema generator infers []byte (array of ints)\n"+
				"  Fix: change the field type to any (or []any), then convert with types.ToAny:\n"+
				"    v, err := types.ToAny(typedValue)\n"+
				"    output.Field = v",
			toolName, elem, strings.Join(paths, ", "),
		))
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

// rawMessageType is the reflect.Type for json.RawMessage.
var rawMessageType = reflect.TypeFor[json.RawMessage]()

// findRawMessageFields recursively walks a struct type and returns field paths
// that use json.RawMessage. This catches schema/runtime mismatches where the
// schema generator infers []byte but json.Marshal produces arbitrary JSON.
func findRawMessageFields(t reflect.Type, path []string, visited map[reflect.Type]bool) []string {
	// Unwrap pointer.
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Direct match: json.RawMessage itself (e.g. element of []json.RawMessage).
	if t == rawMessageType {
		return []string{strings.Join(path, ".")}
	}

	// Prevent infinite recursion on recursive types.
	if visited[t] {
		return nil
	}
	visited[t] = true
	defer delete(visited, t)

	var found []string

	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}

			ft := f.Type
			// Unwrap pointer for the type check.
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}

			fieldPath := append(path, f.Name)

			if ft == rawMessageType {
				found = append(found, strings.Join(fieldPath, "."))
				continue
			}

			found = append(found, findRawMessageFields(ft, fieldPath, visited)...)
		}

	case reflect.Slice, reflect.Array:
		found = append(found, findRawMessageFields(t.Elem(), append(path, "[]"), visited)...)

	case reflect.Map:
		found = append(found, findRawMessageFields(t.Elem(), append(path, "[value]"), visited)...)
	}

	return found
}
