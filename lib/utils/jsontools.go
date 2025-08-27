/*
Copyright 2014 The Kubernetes Authors.

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
	"errors"
	"io"
	"reflect"
	"unicode"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/internalutils/stream"
)

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

// FastUnmarshal uses the json-iterator library for fast JSON unmarshalling.
// Note, this function marshals floats with 6 digits precision.
func FastUnmarshal(data []byte, v any) error {
	iter := jsoniter.ConfigFastest.BorrowIterator(data)
	defer jsoniter.ConfigFastest.ReturnIterator(iter)

	iter.ReadVal(v)
	if iter.Error != nil {
		return trace.Wrap(iter.Error)
	}

	return nil
}

// SafeConfig uses jsoniter's ConfigFastest settings but enables map key
// sorting to ensure CompareAndSwap checks consistently succeed.
var SafeConfig = jsoniter.Config{
	EscapeHTML:                    false,
	MarshalFloatWith6Digits:       true, // will lose precision
	ObjectFieldMustBeSimpleString: true, // do not unescape object field
	SortMapKeys:                   true,
}.Froze()

// SafeConfigWithIndent is equivalent to SafeConfig except with indentation
// enabled.
var SafeConfigWithIndent = jsoniter.Config{
	IndentionStep:                 2,
	EscapeHTML:                    false,
	MarshalFloatWith6Digits:       true, // will lose precision
	ObjectFieldMustBeSimpleString: true, // do not unescape object field
	SortMapKeys:                   true,
}.Froze()

// FastMarshal uses the json-iterator library for fast JSON marshaling.
// Note, this function unmarshals floats with 6 digits precision.
func FastMarshal(v any) ([]byte, error) {
	data, err := SafeConfig.Marshal(v)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

// FastMarshal uses the json-iterator library for fast JSON marshaling
// with indentation. Note, this function unmarshals floats with 6 digits precision.
func FastMarshalIndent(v any, prefix, indent string) ([]byte, error) {
	data, err := SafeConfig.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

// WriteJSONArray marshals values as a JSON array.
func WriteJSONArray[T any](w io.Writer, values []T) error {
	if len(values) == 0 {
		_, err := w.Write([]byte("[]"))
		return err
	}
	return WriteJSON(w, values)
}

// WriteJSONObject marshals m as a JSON object.
func WriteJSONObject[M ~map[K]V, K comparable, V any](w io.Writer, m M) error {
	if len(m) == 0 {
		_, err := w.Write([]byte("{}"))
		return err
	}
	return WriteJSON(w, m)
}

// WriteJSON marshals multiple documents as a JSON list with indentation.
func WriteJSON(w io.Writer, values any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	err := encoder.Encode(values)
	return trace.Wrap(err)
}

// StremJSONArray streams the elements of a stream.Stream as a json array
// with optional indentation (used to stream to CLI).
func StreamJSONArray[T any](items stream.Stream[T], out io.Writer, indent bool) error {
	cfg := SafeConfig
	if indent {
		cfg = SafeConfigWithIndent
	}
	stream := jsoniter.NewStream(cfg, out, 512)
	stream.WriteArrayStart()
	var prev bool
	for items.Next() {
		if prev {
			// if a previous item was written to the array, we need to
			// write a comma first.
			stream.WriteMore()
		}
		stream.WriteVal(items.Item())
		prev = true
	}
	stream.WriteArrayEnd()
	return trace.NewAggregate(items.Done(), stream.Flush())
}

const yamlDocDelimiter = "---"

// WriteYAML detects whether value is a list
// and marshals multiple documents delimited by `---`, otherwise, marshals
// a single value
func WriteYAML(w io.Writer, values any) error {
	if reflect.TypeOf(values).Kind() != reflect.Slice {
		return trace.Wrap(writeYAML(w, values))
	}
	// first pass makes sure that all values are documents (objects or maps)
	slice := reflect.ValueOf(values)
	if slice.Len() == 0 {
		_, err := w.Write([]byte("[]"))
		return err
	}

	allDocs := func() bool {
		for i := range slice.Len() {
			if !isDoc(slice.Index(i)) {
				return false
			}
		}
		return true
	}
	if !allDocs() {
		return trace.Wrap(writeYAML(w, values))
	}
	// second pass can marshal documents
	for i := range slice.Len() {
		err := writeYAML(w, slice.Index(i).Interface())
		if err != nil {
			return trace.Wrap(err)
		}
		if i != slice.Len()-1 {
			if _, err := w.Write([]byte(yamlDocDelimiter + "\n")); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// isDoc detects whether value constitutes a document
func isDoc(val reflect.Value) bool {
	iterations := 0
	for val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr {
		val = val.Elem()
		// preventing cycles
		iterations++
		if iterations > 10 {
			return false
		}
	}
	return val.Kind() == reflect.Struct || val.Kind() == reflect.Map
}

// writeYAML writes marshaled YAML to writer
func writeYAML(w io.Writer, values any) error {
	data, err := yaml.Marshal(values)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

// ReadYAML can unmarshal a stream of documents, used in tests.
func ReadYAML(reader io.Reader) (any, error) {
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	var values []any
	for {
		var val any
		err := decoder.Decode(&val)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(values) == 0 {
					return nil, trace.BadParameter("no resources found, empty input?")
				}
				if len(values) == 1 {
					return values[0], nil
				}
				return values, nil
			}
			return nil, trace.Wrap(err)
		}
		values = append(values, val)
	}
}
