// Adapted from github.com/oapi-codegen/runtime v1.4.0 (styleparam.go). The
// structure and comments of the upstream file are preserved; the only
// substantive changes are:
//
//   - The TextMarshaler carve-out no longer checks `types.Date`, since this
//     subset has dropped the upstream `github.com/oapi-codegen/runtime/types`
//     dependency.
//   - The `reflect.Slice`, `reflect.Struct` (other than time.Time / uuid.UUID),
//     and `reflect.Map` branches of the top-level `switch t.Kind()` return an
//     error rather than dispatching to `styleSlice` / `styleStruct` /
//     `styleMap`, since those helpers are not vendored here.
//   - `marshalKnownTypes` no longer handles `types.Date`.
//
// See the package README for the full subset and the corresponding error
// assertions in `styleparam_test.go`.
//
// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oapiruntime

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Parameter escaping works differently based on where a header is found

type ParamLocation int

const (
	ParamLocationUndefined ParamLocation = iota
	ParamLocationQuery
	ParamLocationPath
	ParamLocationHeader
	ParamLocationCookie
)

// StyleParam is used by older generated code, and must remain compatible
// with that code. It is not to be used in new templates. Please see the
// function below, which can specialize its output based on the location of
// the parameter.
func StyleParam(style string, explode bool, paramName string, value interface{}) (string, error) {
	return StyleParamWithLocation(style, explode, paramName, ParamLocationUndefined, value)
}

// StyleParamWithLocation serializes a Go value into an OpenAPI-styled parameter
// string, performing escaping based on parameter location.
func StyleParamWithLocation(style string, explode bool, paramName string, paramLocation ParamLocation, value interface{}) (string, error) {
	return StyleParamWithOptions(style, explode, paramName, value, StyleParamOptions{
		ParamLocation: paramLocation,
	})
}

// StyleParamOptions defines optional arguments for StyleParamWithOptions.
type StyleParamOptions struct {
	// ParamLocation controls URL escaping behavior.
	ParamLocation ParamLocation
	// Type is the OpenAPI type of the parameter (e.g. "string", "integer").
	Type string
	// Format is the OpenAPI format of the parameter (e.g. "byte", "date-time").
	// When set to "byte" and the value is []byte, it is base64-encoded as a
	// single string rather than treated as a generic slice of uint8.
	Format string
	// Required indicates whether the parameter is required.
	Required bool
	// AllowReserved, when true, prevents percent-encoding of RFC 3986
	// reserved characters in query parameter values. Per the OpenAPI 3.x
	// spec, this only applies to query parameters.
	AllowReserved bool
}

// StyleParamWithOptions serializes a Go value into an OpenAPI-styled parameter
// string with additional options.
func StyleParamWithOptions(style string, explode bool, paramName string, value interface{}, opts StyleParamOptions) (string, error) {
	t := reflect.TypeOf(value)
	v := reflect.ValueOf(value)

	// Things may be passed in by pointer, we need to dereference, so return
	// error on nil.
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", fmt.Errorf("value is a nil pointer")
		}
		v = reflect.Indirect(v)
		t = v.Type()
	}

	// If the value implements encoding.TextMarshaler we use it for marshaling
	// https://github.com/deepmap/oapi-codegen/issues/504
	if tu, ok := value.(encoding.TextMarshaler); ok {
		t := reflect.Indirect(reflect.ValueOf(value)).Type()
		convertableToTime := t.ConvertibleTo(reflect.TypeOf(time.Time{}))

		// Since time.Time implements encoding.TextMarshaler we should avoid
		// calling its MarshalText(). (Upstream also excluded types.Date here;
		// see the file banner for why that check was dropped.)
		if !convertableToTime {
			b, err := tu.MarshalText()
			if err != nil {
				return "", fmt.Errorf("error marshaling '%s' as text: %w", value, err)
			}

			return stylePrimitive(style, explode, paramName, opts.ParamLocation, opts.AllowReserved, string(b))
		}
	}

	switch t.Kind() {
	case reflect.Slice:
		// Teleport subset: slice styling (including the `[]byte` / `format:
		// "byte"` short-circuit and the general `styleSlice` dispatch) is
		// not vendored. Port the upstream logic over if the generated client
		// starts emitting slice parameters.
		return "", fmt.Errorf("unsupported kind %s; see the oapiruntime package README", t.Kind())
	case reflect.Struct:
		// Teleport subset: only time.Time and uuid.UUID (and their named
		// aliases) are supported; `styleStruct` (generic struct / deepObject /
		// json.Marshaler dispatch) is not vendored.
		if s, ok := marshalKnownTypes(value); ok {
			return stylePrimitive(style, explode, paramName, opts.ParamLocation, opts.AllowReserved, s)
		}
		return "", fmt.Errorf("unsupported struct type %s; see the oapiruntime package README", t.String())
	case reflect.Map:
		// Teleport subset: `styleMap` is not vendored.
		return "", fmt.Errorf("unsupported kind %s; see the oapiruntime package README", t.Kind())
	default:
		return stylePrimitive(style, explode, paramName, opts.ParamLocation, opts.AllowReserved, value)
	}
}

// These are special cases. The value may be a date, time, or uuid,
// in which case, marshal it into the correct format. (The types.Date branch
// was dropped in this Teleport subset — see the file banner.)
func marshalKnownTypes(value interface{}) (string, bool) {
	v := reflect.Indirect(reflect.ValueOf(value))
	t := v.Type()

	if t.ConvertibleTo(reflect.TypeOf(time.Time{})) {
		tt := v.Convert(reflect.TypeOf(time.Time{}))
		timeVal := tt.Interface().(time.Time)
		return timeVal.Format(time.RFC3339Nano), true
	}

	if t.ConvertibleTo(reflect.TypeOf(uuid.UUID{})) {
		u := v.Convert(reflect.TypeOf(uuid.UUID{}))
		uuidVal := u.Interface().(uuid.UUID)
		return uuidVal.String(), true
	}

	return "", false
}

func stylePrimitive(style string, explode bool, paramName string, paramLocation ParamLocation, allowReserved bool, value interface{}) (string, error) {
	strVal, err := primitiveToString(value)
	if err != nil {
		return "", err
	}

	escapedName := escapeParameterName(paramName, paramLocation)

	var prefix string
	switch style {
	case "simple":
	case "label":
		prefix = "."
	case "matrix":
		prefix = fmt.Sprintf(";%s=", escapedName)
	case "form":
		prefix = fmt.Sprintf("%s=", escapedName)
	default:
		return "", fmt.Errorf("unsupported style '%s'", style)
	}
	return prefix + escapeParameterString(strVal, paramLocation, allowReserved), nil
}

// Converts a primitive value to a string. We need to do this based on the
// Kind of an interface, not the Type to work with aliased types.
func primitiveToString(value interface{}) (string, error) {
	var output string

	// sometimes time and date used like primitive types
	// it can happen if paramether is object and has time or date as field
	if res, ok := marshalKnownTypes(value); ok {
		return res, nil
	}

	// Values may come in by pointer for optionals, so make sure to dereferene.
	v := reflect.Indirect(reflect.ValueOf(value))
	t := v.Type()
	kind := t.Kind()

	switch kind {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		output = strconv.FormatInt(v.Int(), 10)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		output = strconv.FormatUint(v.Uint(), 10)
	case reflect.Float64:
		output = strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Float32:
		output = strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Bool:
		if v.Bool() {
			output = "true"
		} else {
			output = "false"
		}
	case reflect.String:
		output = v.String()
	case reflect.Struct:
		// If input has Marshaler, such as object has Additional Property or AnyOf,
		// We use this Marshaler and convert into interface{} before styling.
		if v, ok := value.(uuid.UUID); ok {
			output = v.String()
			break
		}
		if m, ok := value.(json.Marshaler); ok {
			buf, err := m.MarshalJSON()
			if err != nil {
				return "", fmt.Errorf("failed to marshal input to JSON: %w", err)
			}
			e := json.NewDecoder(bytes.NewReader(buf))
			e.UseNumber()
			var i2 interface{}
			err = e.Decode(&i2)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
			}
			output, err = primitiveToString(i2)
			if err != nil {
				return "", fmt.Errorf("error convert JSON structure: %w", err)
			}
			break
		}
		fallthrough
	default:
		v, ok := value.(fmt.Stringer)
		if !ok {
			return "", fmt.Errorf("unsupported type %s", reflect.TypeOf(value).String())
		}

		output = v.String()
	}
	return output, nil
}

// escapeParameterName escapes a parameter name for use in query strings and
// paths. This ensures characters like [] in parameter names (e.g. user_ids[])
// are properly percent-encoded per RFC 3986.
func escapeParameterName(name string, paramLocation ParamLocation) string {
	// Parameter names should always be encoded regardless of allowReserved,
	// which only applies to values per the OpenAPI spec.
	return escapeParameterString(name, paramLocation, false)
}

// escapeParameterString escapes a parameter value based on the location of
// that parameter. Query params and path params need different kinds of
// escaping, while header and cookie params seem not to need escaping.
// When allowReserved is true and the location is query, RFC 3986 reserved
// characters are left unencoded per the OpenAPI allowReserved specification.
func escapeParameterString(value string, paramLocation ParamLocation, allowReserved bool) string {
	switch paramLocation {
	case ParamLocationQuery:
		if allowReserved {
			return escapeQueryAllowReserved(value)
		}
		return url.QueryEscape(value)
	case ParamLocationPath:
		return url.PathEscape(value)
	default:
		return value
	}
}

// escapeQueryAllowReserved percent-encodes a query parameter value while
// leaving RFC 3986 reserved characters (:/?#[]@!$&'()*+,;=) unencoded, as
// specified by OpenAPI's allowReserved parameter option. Only characters that
// are neither unreserved nor reserved are encoded (e.g., spaces, control
// characters, non-ASCII).
func escapeQueryAllowReserved(value string) string {
	// RFC 3986 reserved characters that should NOT be encoded when
	// allowReserved is true.
	const reserved = `:/?#[]@!$&'()*+,;=`

	var buf strings.Builder
	for _, b := range []byte(value) {
		if isUnreserved(b) || strings.IndexByte(reserved, b) >= 0 {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

// isUnreserved reports whether the byte is an RFC 3986 unreserved character:
// ALPHA / DIGIT / "-" / "." / "_" / "~"
func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}
