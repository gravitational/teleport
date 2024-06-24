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

func (s *SignatureAlgorithmSuite) toString() string {
	switch *s {
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY:
		return "legacy"
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1:
		return "balanced-v1"
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1:
		return "fips-v1"
	case SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1:
		return "hsm-v1"
	default:
		return s.String()
	}
}

func (s *SignatureAlgorithmSuite) fromString(str string) error {
	switch str {
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

// UnmarshalJSON marshals a SignatureAlgorithmSuite value to JSON.
func (s *SignatureAlgorithmSuite) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.toString() + `"`), nil
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
		return trace.Wrap(s.fromString(v))
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
