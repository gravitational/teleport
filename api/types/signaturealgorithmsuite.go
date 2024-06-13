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

package types

import (
	"encoding/json"
	"strings"

	"github.com/gravitational/trace"
)

// MarshalJSON returns custom strings for each SignatureAlgorithmSuite enum value that match the format
// defined in the RFD: legacy, balanced-v1, fips-v1, legacy-v1, etc. instead of the protobuf BALANCED_V1
// format.
func (s *SignatureAlgorithmSuite) MarshalJSON() ([]byte, error) {
	str := strings.ToLower(s.String())
	str = strings.ReplaceAll(str, "_", "-")
	return []byte(`"` + str + `"`), nil
}

// UnmarshalJSON unmarshals a SignatureAlgorithmSuite and suppports the custom string format or numeric types
// matching an enum value.
func (s *SignatureAlgorithmSuite) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return trace.Wrap(err)
	}
	switch v := val.(type) {
	case string:
		v = strings.ToUpper(v)
		v = strings.ReplaceAll(v, "-", "_")
		suite, ok := SignatureAlgorithmSuite_value[v]
		if !ok {
			return trace.BadParameter("SignatureAlgorithmSuite invalid value %v", v)
		}
		*s = SignatureAlgorithmSuite(suite)
		return nil
	case int32:
		return trace.Wrap(s.setFromEnum(v))
	case int64:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case int:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case float64:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case float32:
		return trace.Wrap(s.setFromEnum(int32(v)))
	default:
		return trace.BadParameter("SignatureAlgorithmSuite invalid type %T", val)
	}
}

func (s *SignatureAlgorithmSuite) setFromEnum(val int32) error {
	if _, ok := SignatureAlgorithmSuite_name[val]; !ok {
		return trace.BadParameter("SignatureAlgorithmSuite invalid value %v", val)
	}
	*s = SignatureAlgorithmSuite(val)
	return nil
}
