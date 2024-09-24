// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yaml

import (
	"bytes"
	"io"

	ghodssyaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	gopkgyaml "gopkg.in/yaml.v3"
)

// A TypeError is returned by Unmarshal when one or more fields in the YAML
// document cannot be properly decoded into the requested types. When this
// error is returned, the value is still unmarshaled partially.
type TypeError = gopkgyaml.TypeError

// Node represents an element in the YAML document hierarchy. See
// gopkg.in/yaml.v3.Node for more information.
type Node = gopkgyaml.Node

// Marshal marshals the object into JSON then converts JSON to YAML and returns
// the YAML.
func Marshal(o any) ([]byte, error) {
	out, err := ghodssyaml.Marshal(o)
	return out, trace.Wrap(err)
}

// Unmarshal Converts YAML to JSON then uses JSON to unmarshal into an object.
func Unmarshal(data []byte, o any) error {
	return trace.Wrap(ghodssyaml.Unmarshal(data, o))
}

// UnmarshalStrict is like Unmarshal except that any fields that are found in
// the data that do not have corresponding struct members, or mapping keys that
// are duplicates, will result in an error.
func UnmarshalStrict(data []byte, o any) error {
	buf := bytes.NewBuffer(data)
	decoder := NewDecoder(buf)
	decoder.KnownFields(true)
	return trace.Wrap(decoder.Decode(o))
}

// JSONToYAML converts JSON to YAML.
func JSONToYAML(data []byte) ([]byte, error) {
	out, err := ghodssyaml.JSONToYAML(data)
	return out, trace.Wrap(err)
}

// YAMLToJSON converts YAML to JSON. See github.com/ghodss/yaml.YAMLToJSON for
// more information.
func YAMLToJSON(data []byte) ([]byte, error) {
	out, err := ghodssyaml.YAMLToJSON(data)
	return out, trace.Wrap(err)
}

// Decoder reads and decodes YAML values from an input stream.
type Decoder interface {
	// Decode reads the next YAML-encoded value from its input and stores it in
	// the value pointed to by v.
	Decode(v any) error
	// KnownFields ensures that the keys in decoded mappings to exist as fields
	// in the struct being decoded into.
	KnownFields(enable bool)
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) Decoder {
	return gopkgyaml.NewDecoder(r)
}

// Encoder writes YAML values to an output stream.
type Encoder interface {
	io.Closer
	// Encode writes the YAML encoding of v to the stream.
	Encode(v any) error
	// SetIndent changes the used indentation used when encoding.
	SetIndent(spaces int)
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) Encoder {
	return gopkgyaml.NewEncoder(w)
}
