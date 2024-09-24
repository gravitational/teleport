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

type TypeError = gopkgyaml.TypeError
type Node = gopkgyaml.Node

func Marshal(o any) ([]byte, error) {
	out, err := ghodssyaml.Marshal(o)
	return out, trace.Wrap(err)
}

func Unmarshal(data []byte, o any) error {
	return trace.Wrap(ghodssyaml.Unmarshal(data, o))
}

func UnmarshalStrict(data []byte, o any) error {
	buf := bytes.NewBuffer(data)
	decoder := NewDecoder(buf)
	decoder.KnownFields(true)
	return trace.Wrap(decoder.Decode(o))
}

func JSONToYAML(data []byte) ([]byte, error) {
	out, err := ghodssyaml.JSONToYAML(data)
	return out, trace.Wrap(err)
}

func YAMLToJSON(data []byte) ([]byte, error) {
	out, err := ghodssyaml.YAMLToJSON(data)
	return out, trace.Wrap(err)
}

type Decoder interface {
	Decode(v any) error
	KnownFields(enable bool)
}

func NewDecoder(r io.Reader) Decoder {
	return gopkgyaml.NewDecoder(r)
}

type Encoder interface {
	io.Closer
	Encode(v any) error
	SetIndent(spaces int)
}

func NewEncoder(w io.Writer) Encoder {
	return gopkgyaml.NewEncoder(w)
}
