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
		fields := findNilDefaultFields(elem, nil, make(map[reflect.Type]bool))
		var fix string
		if len(fields) > 0 {
			fix = "  Nil-defaulting fields:\n"
			for _, f := range fields {
				fix += fmt.Sprintf("    %s (%s) — add `%s` to json tag\n", f.path, f.goType, f.fix)
			}
		} else {
			fix = "  Fix: add `omitzero` to nil-defaulting fields, or use pointers\n"
		}
		panic(fmt.Sprintf(
			"AddTool %q: zero value of output type %s fails schema validation:\n%s",
			toolName, elem, fix,
		))
	}
}

// nilDefaultField describes a struct field that serializes as null in its zero value.
type nilDefaultField struct {
	path   string // dotted Go field path, e.g. "Task.Items"
	goType string // Go type, e.g. "[]string" or "*FeedTask"
	fix    string // suggested tag fix
}

// findNilDefaultFields walks a struct type and returns fields whose zero value
// serializes as null (nil slices, nil maps, nil pointers to structs) and that
// lack omitzero/omitempty tags. These fields will fail schema validation because
// the schema expects a concrete type (array/object) but gets null.
func findNilDefaultFields(t reflect.Type, path []string, visited map[reflect.Type]bool) []nilDefaultField {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	if visited[t] {
		return nil
	}
	visited[t] = true
	defer delete(visited, t)

	var found []nilDefaultField
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		// Check if field is omitted when zero/empty.
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		omits := strings.Contains(tag, "omitzero") || strings.Contains(tag, "omitempty")

		fieldPath := append(path, f.Name)
		ft := f.Type

		switch {
		case ft.Kind() == reflect.Slice || ft.Kind() == reflect.Map:
			// nil slice/map → null; schema expects array/object.
			// omitzero omits when the value is the zero value (nil for slices/maps).
			if !omits {
				found = append(found, nilDefaultField{
					path:   strings.Join(fieldPath, "."),
					goType: ft.String(),
					fix:    "omitzero",
				})
			}
		case ft.Kind() == reflect.Pointer:
			// nil pointer → null; schema expects the pointed-to type.
			// omitempty omits when the pointer is nil.
			if !omits {
				found = append(found, nilDefaultField{
					path:   strings.Join(fieldPath, "."),
					goType: ft.String(),
					fix:    "omitempty",
				})
			}
		}

		// Recurse into struct fields (direct or behind pointer).
		inner := ft
		for inner.Kind() == reflect.Pointer {
			inner = inner.Elem()
		}
		if inner.Kind() == reflect.Struct {
			found = append(found, findNilDefaultFields(inner, fieldPath, visited)...)
		}
	}
	return found
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
