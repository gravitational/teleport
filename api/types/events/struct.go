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

package events

import (
	"bytes"
	"encoding/json"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility
	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
)

// Struct is a wrapper around types.Struct
// that marshals itself into json
type Struct struct {
	types.Struct
}

// decodeToMap converts a pb.Struct to a map from strings to Go types.
func decodeToMap(s *types.Struct) (map[string]interface{}, error) {
	if s == nil {
		return nil, nil
	}
	m := map[string]interface{}{}
	for k, v := range s.Fields {
		var err error
		m[k], err = decodeValue(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return m, nil
}

// decodeValue decodes proto value to golang type
func decodeValue(v *types.Value) (interface{}, error) {
	switch k := v.Kind.(type) {
	case *types.Value_NullValue:
		return nil, nil
	case *types.Value_NumberValue:
		return k.NumberValue, nil
	case *types.Value_StringValue:
		return k.StringValue, nil
	case *types.Value_BoolValue:
		return k.BoolValue, nil
	case *types.Value_StructValue:
		return decodeToMap(k.StructValue)
	case *types.Value_ListValue:
		s := make([]interface{}, len(k.ListValue.Values))
		for i, e := range k.ListValue.Values {
			var err error
			s[i], err = decodeValue(e)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return s, nil
	default:
		return nil, trace.BadParameter("protostruct: unknown kind %v", k)
	}
}

// MarshalJSON marshals boolean value.
func (s *Struct) MarshalJSON() ([]byte, error) {
	m, err := decodeToMap(&s.Struct)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return json.Marshal(m)
}

// UnmarshalJSON unmarshals JSON from string or bool,
// in case if value is missing or not recognized, defaults to false
func (s *Struct) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader(data), &s.Struct)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// EncodeMap encodes map[string]interface{} to map<string, Value>
func EncodeMap(msg map[string]interface{}) (*Struct, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbs := types.Struct{}
	if err = (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader(data), &pbs); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Struct{Struct: pbs}, nil
}

// EncodeMapStrings encodes map[string][]string to map<string, Value>
func EncodeMapStrings(msg map[string][]string) (*Struct, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pbs := types.Struct{}
	if err = (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader(data), &pbs); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Struct{Struct: pbs}, nil
}

// MustEncodeMap panics if EncodeMap returns error
func MustEncodeMap(msg map[string]interface{}) *Struct {
	m, err := EncodeMap(msg)
	if err != nil {
		panic(err)
	}
	return m
}
