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

	"github.com/gravitational/trace"
)

// MarshalText marshals a SignatureAlgorithmSuite value to text. This gets used
// by json.Marshal.
func (s SignatureAlgorithmSuite) MarshalText() ([]byte, error) {
	switch s {
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY:
		return []byte("legacy"), nil
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1:
		return []byte("balanced-v1"), nil
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1:
		return []byte("fips-v1"), nil
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1:
		return []byte("hsm-v1"), nil
	default:
		return []byte(s.String()), nil
	}
}

// UnmarshalJSON unmarshals a SignatureAlgorithmSuite and supports the custom
// string format or numeric types matching an enum value.
func (s *SignatureAlgorithmSuite) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return trace.Wrap(err)
	}
	switch v := val.(type) {
	case string:
		return trace.Wrap(s.UnmarshalText([]byte(v)))
	case float64:
		// json.Unmarshal is documented to unmarshal any JSON number into an
		// int64 when unmarshaling into an interface.
		return trace.Wrap(s.setFromEnum(int32(v)))
	default:
		return trace.BadParameter("SignatureAlgorithmSuite invalid type %T", val)
	}
}

// UnmarshalText unmarshals a SignatureAlgorithmSuite from text and supports the
// custom string format or the proto enum values. This is used by JSON and YAML
// unmarshallers.
func (s *SignatureAlgorithmSuite) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "":
		*s = SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED
	case "legacy":
		*s = SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY
	case "balanced-v1":
		*s = SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1
	case "fips-v1":
		*s = SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1
	case "hsm-v1":
		*s = SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1
	default:
		if v, ok := SignatureAlgorithmSuite_value[str]; ok {
			*s = SignatureAlgorithmSuite(v)
		} else {
			return trace.BadParameter("SignatureAlgorithmSuite invalid value: %s", str)
		}
	}
	return nil
}

func (s *SignatureAlgorithmSuite) setFromEnum(val int32) error {
	if _, ok := SignatureAlgorithmSuite_name[val]; !ok {
		return trace.BadParameter("SignatureAlgorithmSuite invalid value %v", val)
	}
	*s = SignatureAlgorithmSuite(val)
	return nil
}
