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

	"github.com/gravitational/trace"
)

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
