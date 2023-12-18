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

package types

import (
	"encoding/json"

	"github.com/gravitational/trace"
)

var certExtensionTypeName = map[CertExtensionType]string{
	CertExtensionType_SSH: "ssh",
}

var certExtensionTypeValue = map[string]CertExtensionType{
	"ssh": CertExtensionType_SSH,
}

func (t CertExtensionType) MarshalJSON() ([]byte, error) {
	name, ok := certExtensionTypeName[t]
	if !ok {
		return nil, trace.Errorf("invalid certificate extension type: %q", t)
	}
	return json.Marshal(name)
}

func (t *CertExtensionType) UnmarshalJSON(b []byte) error {
	var anyVal any
	if err := json.Unmarshal(b, &anyVal); err != nil {
		return err
	}

	switch val := anyVal.(type) {
	case string:
		enumVal, ok := certExtensionTypeValue[val]
		if !ok {
			return trace.Errorf("invalid certificate extension type: %q", string(b))
		}
		*t = enumVal
		return nil
	case int32:
		return t.setFromEnum(val)
	case int:
		return t.setFromEnum(int32(val))
	case int64:
		return t.setFromEnum(int32(val))
	case float64:
		return trace.Wrap(t.setFromEnum(int32(val)))
	case float32:
		return trace.Wrap(t.setFromEnum(int32(val)))
	default:
		return trace.BadParameter("unexpected type %T", val)
	}
}

// setFromEnum sets the value from enum value as int32.
func (t *CertExtensionType) setFromEnum(val int32) error {
	if _, ok := CertExtensionType_name[val]; !ok {
		return trace.BadParameter("invalid cert extension mode %v", val)
	}
	*t = CertExtensionType(val)
	return nil
}

var certExtensionModeName = map[CertExtensionMode]string{
	CertExtensionMode_EXTENSION: "extension",
}

var certExtensionModeValue = map[string]CertExtensionMode{
	"extension": CertExtensionMode_EXTENSION,
}

func (t CertExtensionMode) MarshalJSON() ([]byte, error) {
	name, ok := certExtensionModeName[t]
	if !ok {
		return nil, trace.Errorf("invalid certificate extension mode: %q", t)
	}
	return json.Marshal(name)
}

func (t *CertExtensionMode) UnmarshalJSON(b []byte) error {
	var anyVal any
	if err := json.Unmarshal(b, &anyVal); err != nil {
		return err
	}
	switch val := anyVal.(type) {
	case string:
		enumVal, ok := certExtensionModeValue[val]
		if !ok {
			return trace.Errorf("invalid certificate extension mode: %q", string(b))
		}
		*t = enumVal
		return nil
	case int32:
		return t.setFromEnum(val)
	case int:
		return t.setFromEnum(int32(val))
	case int64:
		return t.setFromEnum(int32(val))
	case float64:
		return trace.Wrap(t.setFromEnum(int32(val)))
	case float32:
		return trace.Wrap(t.setFromEnum(int32(val)))
	default:
		return trace.BadParameter("unexpected type %T", val)
	}
}

// setFromEnum sets the value from enum value as int32.
func (t *CertExtensionMode) setFromEnum(val int32) error {
	if _, ok := CertExtensionMode_name[val]; !ok {
		return trace.BadParameter("invalid cert extension mode %v", val)
	}
	*t = CertExtensionMode(val)
	return nil
}
