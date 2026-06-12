/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"encoding/json"
	"strings"

	"github.com/gravitational/trace"
)

// Strings is a list of string that can unmarshal from list of strings
// or a scalar string from scalar yaml or json property
type Strings []string

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

// CopyStrings makes a deep copy of the passed in string slice and returns
// the copy.
func CopyStrings(in []string) []string {
	if in == nil {
		return nil
	}

	out := make([]string, len(in))
	copy(out, in)

	return out
}

// MapToStrings collects keys and values of a map into a slice of strings.
func MapToStrings(m map[string]string) []string {
	s := make([]string, 0, len(m)*2)
	for key, value := range m {
		s = append(s, key, value)
	}
	return s
}

// ToLowerStrings lower cases each string in a slice.
func ToLowerStrings(strs []string) []string {
	lowerCasedStrs := make([]string, len(strs))
	for i, s := range strs {
		lowerCasedStrs[i] = strings.ToLower(s)
	}

	return lowerCasedStrs
}
