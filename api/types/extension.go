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
	var stringVal string
	if err := json.Unmarshal(b, &stringVal); err != nil {
		return err
	}

	val, ok := certExtensionTypeValue[stringVal]
	if !ok {
		return trace.Errorf("invalid certificate extension type: %q", string(b))
	}
	*t = val
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
	var stringVal string
	if err := json.Unmarshal(b, &stringVal); err != nil {
		return err
	}
	val, ok := certExtensionModeValue[stringVal]
	if !ok {
		return trace.Errorf("invalid certificate extension mode: %q", string(b))
	}
	*t = val
	return nil
}
