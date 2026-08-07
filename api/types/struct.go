/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"bytes"
	"fmt"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
)

// Struct is a wrapper around [gogotypes.Struct] mirroring a similar variant in
// api/types/struct.go, which we can't import here as it would cause a cycle.
type Struct struct {
	gogotypes.Struct
}

func (s *Struct) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := (&jsonpb.Marshaler{}).Marshal(&buf, &s.Struct); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func (s *Struct) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	if err := (&jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}).Unmarshal(bytes.NewReader(data), &s.Struct); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GogoStruct returns the inner gogo-style struct
func (s *Struct) GogoStruct() *gogotypes.Struct {
	if s == nil {
		return nil
	}

	return &s.Struct
}

// NewStructFromGogoValues returns a new struct wrapper from an unpacked map of
// [gogotypes.Value], useful for constructing struct types in tests.
func NewStructFromGogoValues(fields map[string]*gogotypes.Value) *Struct {
	return &Struct{
		Struct: gogotypes.Struct{
			Fields: fields,
		},
	}
}

// NewStruct converts a gogo struct to our wrapped variant
func NewStruct(s *gogotypes.Struct) *Struct {
	if s == nil {
		return nil
	}

	return &Struct{Struct: *s}
}

// NewStructFromGoValues creates a Struct from a plain Go map. It supports only
// the gogo primitives (string, number, bool, null, struct, list). It is
// primarily used in tests to more easily construct Struct fields.
func NewStructFromGoValues(m map[string]any) (*Struct, error) {
	inner, err := mapToGogoStruct(m)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Struct{Struct: *inner}, nil
}

// mapToGogoStruct converts a map to a gogo struct
func mapToGogoStruct(m map[string]any) (*gogotypes.Struct, error) {
	// This exists for depguard+forbidigo appeasement by converting and
	// constructing Go values to gogo without touching jsonpb.
	fields := make(map[string]*gogotypes.Value, len(m))
	for k, v := range m {
		val, err := anyToGogoValue(v)
		if err != nil {
			return nil, err
		}

		fields[k] = val
	}

	return &gogotypes.Struct{Fields: fields}, nil
}

// anyToGogoValue converts a JSON-compatible primitive type to a gogo value.
func anyToGogoValue(v any) (*gogotypes.Value, error) {
	switch t := v.(type) {
	case nil:
		return &gogotypes.Value{Kind: &gogotypes.Value_NullValue{}}, nil
	case bool:
		return &gogotypes.Value{Kind: &gogotypes.Value_BoolValue{BoolValue: t}}, nil
	case float64:
		return &gogotypes.Value{Kind: &gogotypes.Value_NumberValue{NumberValue: t}}, nil
	case string:
		return &gogotypes.Value{Kind: &gogotypes.Value_StringValue{StringValue: t}}, nil
	case map[string]any:
		s, err := mapToGogoStruct(t)
		if err != nil {
			return nil, err
		}

		return &gogotypes.Value{Kind: &gogotypes.Value_StructValue{StructValue: s}}, nil
	case []any:
		vals := make([]*gogotypes.Value, len(t))
		for i, item := range t {
			val, err := anyToGogoValue(item)
			if err != nil {
				return nil, err
			}

			vals[i] = val
		}

		return &gogotypes.Value{Kind: &gogotypes.Value_ListValue{
			ListValue: &gogotypes.ListValue{Values: vals},
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported json type %T", v)
	}
}
