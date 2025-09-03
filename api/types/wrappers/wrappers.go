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

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

// Traits is a wrapper around map with string
// slices as values
type Traits map[string][]string

func (l Traits) protoType() *LabelValues {
	v := &LabelValues{
		Values: make(map[string]StringValues, len(l)),
	}
	for key, vals := range l {
		stringValues := StringValues{
			Values: make([]string, len(vals)),
		}
		copy(stringValues.Values, vals)
		v.Values[key] = stringValues
	}
	return v
}

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
		clone[key] = slices.Clone(vals)
	}
	return clone
}

// Marshal marshals value into protobuf representation
func (l Traits) Marshal() ([]byte, error) {
	return proto.Marshal(l.protoType())
}

// MarshalTo marshals value to the array
func (l Traits) MarshalTo(data []byte) (int, error) {
	return l.protoType().MarshalTo(data)
}

// Unmarshal unmarshals value from protobuf
func (l *Traits) Unmarshal(data []byte) error {
	protoValues := &LabelValues{}
	err := proto.Unmarshal(data, protoValues)
	if err != nil {
		return err
	}
	if protoValues.Values == nil {
		return nil
	}
	*l = make(map[string][]string, len(protoValues.Values))
	for key := range protoValues.Values {
		(*l)[key] = protoValues.Values[key].Values
	}
	return nil
}

// Size returns protobuf size
func (l Traits) Size() int {
	return l.protoType().Size()
}

// Strings is a list of string that can unmarshal from list of strings
// or a scalar string from scalar yaml or json property
type Strings []string

func (s *Strings) protoType() *StringValues {
	return &StringValues{
		Values: *s,
	}
}

// Marshal marshals value into protobuf representation
func (s Strings) Marshal() ([]byte, error) {
	return proto.Marshal(s.protoType())
}

// MarshalTo marshals value to the array
func (s Strings) MarshalTo(data []byte) (int, error) {
	return s.protoType().MarshalTo(data)
}

// Unmarshal unmarshals value from protobuf
func (s *Strings) Unmarshal(data []byte) error {
	protoValues := &StringValues{}
	err := proto.Unmarshal(data, protoValues)
	if err != nil {
		return err
	}
	if protoValues.Values != nil {
		*s = protoValues.Values
	}
	return nil
}

// Size returns protobuf size
func (s Strings) Size() int {
	return s.protoType().Size()
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
