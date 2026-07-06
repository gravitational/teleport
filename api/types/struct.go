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
