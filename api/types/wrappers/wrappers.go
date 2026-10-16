/*
Copyright 2019 Gravitational, Inc.

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

// Package wrappers provides protobuf wrappers for common teleport map and list types.
package wrappers

import (
	"encoding/json"
	"slices"

	"github.com/gravitational/trace"

	internalwrappers "github.com/gravitational/teleport/api/types/internal/wrappers"
)

// Traits is a wrapper around map with string
// slices as values
type Traits map[string][]string

var _ internalwrappers.GogoprotoCustomType = (*Traits)(nil)

// MarshalTraits will marshal Traits as JSON. Used to embed traits into
// certificates.
func MarshalTraits(traits *Traits) ([]byte, error) {
	return json.Marshal(traits)
}

// UnmarshalTraits will unmarshal JSON traits. Used to embed traits into
// certificates.
func UnmarshalTraits(data []byte, traits *Traits) error {
	err := json.Unmarshal(data, traits)
	if err != nil {
		return traits.Unmarshal(data)
	}
	return nil
}

// Clone returns a copy of the Traits map.
func (l Traits) Clone() Traits {
	if l == nil {
		return nil
	}
	clone := make(Traits, len(l))
	for key, vals := range l {
		if len(vals) == 0 || len(vals) == 1 && vals[0] == "" {
			continue
		}
		clone[key] = slices.Clone(vals)
	}
	return clone
}

// Size implements [internalwrappers.GogoprotoCustomType].
func (l Traits) Size() int {
	return internalwrappers.LabelValuesSizeFromMap(l)
}

// Marshal implements [internalwrappers.GogoprotoCustomType].
func (l Traits) Marshal() ([]byte, error) {
	return internalwrappers.LabelValuesMarshalFromMap(l)
}

// MarshalTo implements [internalwrappers.GogoprotoCustomType].
func (l Traits) MarshalTo(data []byte) (int, error) {
	return internalwrappers.LabelValuesMarshalToFromMap(l, data)
}

// Unmarshal implements [internalwrappers.GogoprotoCustomType].
func (l *Traits) Unmarshal(data []byte) error {
	protoValues := new(LabelValues)
	if err := protoValues.Unmarshal(data); err != nil {
		return trace.Wrap(err)
	}
	if protoValues.Values == nil {
		return nil
	}
	newValues := make(Traits, len(protoValues.Values))
	for key, values := range protoValues.Values {
		newValues[key] = values.Values
	}
	*l = newValues
	return nil
}

// Strings is a list of string that can unmarshal from list of strings
// or a scalar string from scalar yaml or json property
type Strings []string

var _ internalwrappers.GogoprotoCustomType = (*Strings)(nil)

// Size implements [internalwrappers.GogoprotoCustomType].
func (s Strings) Size() int {
	return (&StringValues{Values: s}).Size()
}

// Marshal implements [internalwrappers.GogoprotoCustomType].
func (s Strings) Marshal() ([]byte, error) {
	return (&StringValues{Values: s}).Marshal()
}

// MarshalTo implements [internalwrappers.GogoprotoCustomType].
func (s Strings) MarshalTo(data []byte) (int, error) {
	return (&StringValues{Values: s}).MarshalTo(data)
}

// Unmarshal implements [internalwrappers.GogoprotoCustomType].
func (s *Strings) Unmarshal(data []byte) error {
	protoValues := new(StringValues)
	if err := protoValues.Unmarshal(data); err != nil {
		return trace.Wrap(err)
	}
	if protoValues.Values != nil {
		*s = protoValues.Values
	}
	return nil
}

// UnmarshalJSON unmarshals scalar string or strings slice to Strings
func (s *Strings) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err == nil {
		*s = []string{stringVar}
		return nil
	}
	var stringsVar []string
	if err := json.Unmarshal(data, &stringsVar); err != nil {
		return trace.Wrap(err)
	}
	*s = stringsVar
	return nil
}

// UnmarshalYAML is used to allow Strings to unmarshal from
// scalar string value or from the list
func (s *Strings) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// try unmarshal as string
	var val string
	err := unmarshal(&val)
	if err == nil {
		*s = []string{val}
		return nil
	}

	// try unmarshal as slice
	var slice []string
	err = unmarshal(&slice)
	if err == nil {
		*s = slice
		return nil
	}

	return err
}

// MarshalJSON marshals to scalar value
// if there is only one value in the list
// to list otherwise
func (s Strings) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// MarshalYAML marshals to scalar value
// if there is only one value in the list,
// marshals to list otherwise
func (s Strings) MarshalYAML() (interface{}, error) {
	if len(s) == 1 {
		return s[0], nil
	}
	return []string(s), nil
}
