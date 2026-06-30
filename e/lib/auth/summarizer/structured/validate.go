package structured

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// Validate returns nil if data is a JSON document matching the schema implied by the Go type T together with its
// `json` and `jsonschema` struct tags, and a trace.BadParameter describing every mismatch otherwise. Every field is a
// property named by its `json` tag; fields tagged `jsonschema:"required"` must be present; `jsonschema:"enum=..."`
// restricts the permitted values; no properties beyond the declared fields are allowed (additionalProperties:false);
// and each value must have the JSON type of its Go field. Fields tagged `jsonschema:"-"` (server-populated, never
// produced by the model) are ignored.
//
// The raw JSON is validated rather than a value unmarshaled into T, because a struct's zero values would
// satisfy "required" and mask empty or partial responses.
func Validate[T any](data []byte) error {
	instance, err := decodeJSON(data)
	if err != nil {
		return trace.Wrap(err)
	}

	t := reflect.TypeFor[T]()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	var p problems
	p.checkValue(t, instance, "response", nil)

	if len(p) > 0 {
		return trace.BadParameter("response does not match schema: %s", strings.Join(p, "; "))
	}

	return nil
}

// decodeJSON unmarshals JSON into the generic any model, decoding numbers as json.Number so integer fields can be
// validated exactly.
func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, trace.Wrap(err)
	}

	return v, nil
}

// fieldSchema extracts the property name and constraints a struct field contributes to the schema, mirroring how the
// schema generator interprets the tags. skip is true for fields excluded from the schema (unexported, json:"-", or jsonschema:"-").
func fieldSchema(field reflect.StructField) (name string, required bool, enum []string, skip bool) {
	if !field.IsExported() {
		return "", false, nil, true
	}

	jsTag := field.Tag.Get("jsonschema")
	if jsTag == "-" {
		return "", false, nil, true
	}

	name, _, _ = strings.Cut(field.Tag.Get("json"), ",")
	if name == "-" {
		return "", false, nil, true
	}

	if name == "" {
		// No json tag: the generator falls back to the Go field name.
		name = field.Name
	}

	for _, opt := range strings.Split(jsTag, ",") {
		switch {
		case opt == "required":
			required = true
		case strings.HasPrefix(opt, "enum="):
			enum = append(enum, strings.TrimPrefix(opt, "enum="))
		}
	}

	return name, required, enum, false
}

// isIntegral reports whether the JSON number n has no fractional part, so it satisfies an "integer" field.
// 5 and 5.0 are integral; 5.5 is not.
func isIntegral(n json.Number) bool {
	if _, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
		return true
	}

	f, err := n.Float64()
	if err != nil {
		return false
	}

	return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f)
}

func jsonTypeName(v any) string {
	switch n := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if isIntegral(n) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func renderEnum(enum []string) string {
	quoted := make([]string, len(enum))
	for i, e := range enum {
		quoted[i] = strconv.Quote(e)
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

// problems accumulates the schema mismatches found while walking a value, so the whole document is checked in one pass
// rather than failing at the first mismatch.
type problems []string

func (p *problems) addf(format string, args ...any) {
	*p = append(*p, fmt.Sprintf(format, args...))
}

func (p *problems) addType(loc, want string, v any) {
	p.addf("%s: expected %s but got %s", loc, want, jsonTypeName(v))
}

// checkValue validates a decoded JSON value v against the Go type t. enum, when non-empty, is the set of permitted
// values declared on the struct field v came from.
func (p *problems) checkValue(t reflect.Type, v any, loc string, enum []string) {
	switch t.Kind() {
	case reflect.Pointer:
		// A pointer field is nullable: null is acceptable, otherwise validate the element.
		if v == nil {
			return
		}

		p.checkValue(t.Elem(), v, loc, enum)

	case reflect.Struct:
		p.checkObject(t, v, loc)

	case reflect.Slice, reflect.Array:
		p.checkArray(t, v, loc)

	case reflect.String:
		s, ok := v.(string)
		if !ok {
			p.addType(loc, "string", v)

			return
		}

		if len(enum) > 0 && !slices.Contains(enum, s) {
			p.addf("%s: %q is not one of the permitted values %s", loc, s, renderEnum(enum))
		}

	case reflect.Bool:
		if _, ok := v.(bool); !ok {
			p.addType(loc, "boolean", v)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, ok := v.(json.Number); !ok || !isIntegral(n) {
			p.addType(loc, "integer", v)
		}

	case reflect.Float32, reflect.Float64:
		if _, ok := v.(json.Number); !ok {
			p.addType(loc, "number", v)
		}

	default:
		// Kinds the schema generator never emits (interfaces, maps, channels, ...)
		// impose no constraint here rather than risk a false rejection.
	}
}

// checkObject validates that v is a JSON object whose properties match the exported fields of struct type t: required
// fields are present, declared values validate, and no additional properties appear.
func (p *problems) checkObject(t reflect.Type, v any, loc string) {
	obj, ok := v.(map[string]any)
	if !ok {
		p.addType(loc, "object", v)

		return
	}

	known := make(map[string]struct{}, t.NumField())

	for i := range t.NumField() {
		field := t.Field(i)
		name, required, enum, skip := fieldSchema(field)
		if skip {
			continue
		}

		known[name] = struct{}{}

		val, present := obj[name]
		if !present {
			if required {
				p.addf("%s: missing required property %q", loc, name)
			}

			continue
		}

		p.checkValue(field.Type, val, loc+"."+name, enum)
	}

	// additionalProperties:false - schemagen forbids any property beyond the declared fields. Report unexpected keys in
	// sorted order for stable, deterministic errors.
	var extra []string
	for key := range obj {
		if _, ok := known[key]; !ok {
			extra = append(extra, key)
		}
	}

	slices.Sort(extra)

	for _, key := range extra {
		p.addf("%s: property %q is not allowed", loc, key)
	}
}

// checkArray validates that v is a JSON array and that every element matches the element type of slice/array type t.
func (p *problems) checkArray(t reflect.Type, v any, loc string) {
	arr, ok := v.([]any)
	if !ok {
		p.addType(loc, "array", v)

		return
	}

	elem := t.Elem()
	for i, e := range arr {
		p.checkValue(elem, e, fmt.Sprintf("%s[%d]", loc, i), nil)
	}
}
