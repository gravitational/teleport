/*
Copyright 2022 Gravitational, Inc.

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

// NewSSODiagnosticInfo creates new SSODiagnosticInfo object using arbitrary value, which is serialized using JSON.
func NewSSODiagnosticInfo(infoType SSOInfoType, value interface{}) (*SSODiagnosticInfo, error) {
	out, err := json.Marshal(value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SSODiagnosticInfo{InfoType: infoType, Value: out}, nil
}

// GetValue deserializes embedded JSON of SSODiagnosticInfo.Value with no assumption about underlying type.
func (m *SSODiagnosticInfo) GetValue() (interface{}, error) {
	var value interface{}
	err := json.Unmarshal(m.Value, &value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return value, nil
}

// GetValueTyped deserializes embedded JSON of SSODiagnosticInfo.Value given typed pointer.
func (m *SSODiagnosticInfo) GetValueTyped(value interface{}) error {
	return trace.Wrap(json.Unmarshal(m.Value, &value))
}
