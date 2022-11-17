/*
Copyright 2015 Gravitational, Inc.

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
	"bytes"
	"encoding/json"
	"math"
	"strconv"

	"github.com/gravitational/trace"
)

// IntToUInt16 converts an int to uint16, checking for possible overflow or underflow issues.
func IntToUInt16(i int) (uint16, error) {
	if i > math.MaxUint16 {
		return uint16(math.MaxUint16), trace.Errorf("value too large to cast to uint16: %v", i)
	} else if i < 0 {
		return 0, trace.Errorf("can't cast negative value to uint16: %v", i)
	}
	return uint16(i), nil
}

// IntToUInt32 converts an int to uint32, checking for possible overflow or underflow issues.
func IntToUInt32(i int) (uint32, error) {
	if i > math.MaxUint32 {
		return uint32(math.MaxUint32), trace.Errorf("value too large to cast to uint32: %v", i)
	} else if i < 0 {
		return 0, trace.Errorf("can't cast negative value to uint32: %v", i)
	}
	return uint32(i), nil
}

// IntToInt16 converts an int to int16, checking for possible overflow or underflow issues.
func IntToInt16(i int) (int16, error) {
	if i > math.MaxInt16 {
		return int16(math.MaxInt16), trace.Errorf("value too large to cast to int16: %v", i)
	} else if i < math.MinInt16 {
		return int16(math.MinInt16), trace.Errorf("value too small to cast to int16: %v", i)
	}
	return int16(i), nil
}

// IntToInt32 converts an int to int32, checking for possible overflow or underflow issues.
func IntToInt32(i int) (int32, error) {
	if i > math.MaxInt32 {
		return int32(math.MaxInt32), trace.Errorf("value too large to cast to int32: %v", i)
	} else if i < math.MinInt32 {
		return int32(math.MinInt32), trace.Errorf("value too small to cast to int32: %v", i)
	}
	return int32(i), nil
}

// StrToUInt16 converts a string to uint16, checking for possible overflow or underflow issues.
func StrToUInt16(str string) (uint16, error) {
	i, err := strconv.Atoi(str)
	if err != nil {
		return uint16(0), trace.Errorf("Failed to parse int from string: %s", str)
	}
	return IntToUInt16(i)
}

// StrToUInt32 converts a string to uint32, checking for possible overflow or underflow issues.
func StrToUInt32(str string) (uint32, error) {
	i, err := strconv.Atoi(str)
	if err != nil {
		return uint32(0), trace.Errorf("Failed to parse int from string: %s", str)
	}
	return IntToUInt32(i)
}

// StrToInt16 converts a string to int16, checking for possible overflow or underflow issues.
func StrToInt16(str string) (int16, error) {
	i, err := strconv.Atoi(str)
	if err != nil {
		return int16(0), trace.Errorf("Failed to parse int from string: %s", str)
	}
	return IntToInt16(i)
}

// StrToInt32 converts a string to int32, checking for possible overflow or underflow issues.
func StrToInt32(str string) (int32, error) {
	i, err := strconv.Atoi(str)
	if err != nil {
		return int32(0), trace.Errorf("Failed to parse int from string: %s", str)
	}
	return IntToInt32(i)
}

// ObjectToStruct is converts any structure into JSON and then unmarshalls it into
// another structure.
//
// Teleport configuration uses this (strange, at first) trick to convert from one
// struct type to another, if their fields are loosely compatible via their `json` tags
//
// Example: assume you have two structs:
//
//	type A struct {
//	    Name string `json:"name"`
//		   Age  int    `json:"age"`
//	}
//
//	type B struct {
//		   FullName string `json:"name"`
//	}
//
// Now you can convert B to A:
//
//			b := &B{ FullName: "Bob Dilan"}
//			var a *A
//			utils.ObjectToStruct(b, &a)
//			fmt.Println(a.Name)
//
//	 > "Bob Dilan"
func ObjectToStruct(in interface{}, out interface{}) error {
	bytes, err := json.Marshal(in)
	if err != nil {
		return trace.Wrap(err, "failed to marshal %v, %v", in, err)
	}
	if err := json.Unmarshal(bytes, out); err != nil {
		return trace.Wrap(err, "failed to unmarshal %v into %T, %v", in, out, err)
	}
	return nil
}

// StrictObjectToStruct converts any structure into JSON and then unmarshalls
// it into another structure using a strict decoder.
func StrictObjectToStruct(in interface{}, out interface{}) error {
	data, err := json.Marshal(in)
	if err != nil {
		return trace.Wrap(err, "failed to marshal %v, %v", in, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(out); err != nil {
		return trace.Wrap(err, "failed to unmarshal %v into %T, %v", in, out, err)
	}
	return nil
}
