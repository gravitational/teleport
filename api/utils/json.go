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
	"bytes"
	"encoding/json"
	"unicode"

	"github.com/ghodss/yaml"
	"github.com/gravitational/configure/jsonschema"
	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"
)

// FastUnmarshal uses the json-iterator library for fast JSON unmarshalling.
// Note, this function marshals floats with 6 digits precision.
func FastUnmarshal(data []byte, v interface{}) error {
	iter := jsoniter.ConfigFastest.BorrowIterator(data)
	defer jsoniter.ConfigFastest.ReturnIterator(iter)

	iter.ReadVal(v)
	if iter.Error != nil {
		return trace.Wrap(iter.Error)
	}

	return nil
}

// FastMarshal uses the json-iterator library for fast JSON marshalling.
// Note, this function marshals floats with 6 digits precision.
func FastMarshal(v interface{}) ([]byte, error) {
	data, err := jsoniter.ConfigFastest.Marshal(v)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

// UnmarshalWithSchema processes YAML or JSON encoded object with JSON schema, sets defaults
// and unmarshals resulting object into given struct
func UnmarshalWithSchema(schemaDefinition string, object interface{}, data []byte) error {
	schema, err := jsonschema.New([]byte(schemaDefinition))
	if err != nil {
		return trace.Wrap(err)
	}
	jsonData, err := ToJSON(data)
	if err != nil {
		return trace.Wrap(err)
	}

	raw := map[string]interface{}{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return trace.Wrap(err)
	}
	// schema will check format and set defaults
	processed, err := schema.ProcessObject(raw)
	if err != nil {
		return trace.Wrap(err)
	}
	// since ProcessObject works with unstructured data, the
	// data needs to be re-interpreted in structured form
	bytes, err := json.Marshal(processed)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(bytes, object); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ToJSON converts a single YAML document into a JSON document
// or returns an error. If the document appears to be JSON the
// YAML decoding path is not used (so that error messages are
// JSON specific).
// Creds to: k8s.io for the code
func ToJSON(data []byte) ([]byte, error) {
	if hasJSONPrefix(data) {
		return data, nil
	}
	return yaml.YAMLToJSON(data)
}

var jsonPrefix = []byte("{")

// hasJSONPrefix returns true if the provided buffer appears to start with
// a JSON open brace.
func hasJSONPrefix(buf []byte) bool {
	return hasPrefix(buf, jsonPrefix)
}

// Return true if the first non-whitespace bytes in buf is
// prefix.
func hasPrefix(buf []byte, prefix []byte) bool {
	trim := bytes.TrimLeftFunc(buf, unicode.IsSpace)
	return bytes.HasPrefix(trim, prefix)
}
