/*
Copyright 2026 Gravitational, Inc.

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

import "encoding/json"

// constraintsJSON is the JSON-serialisable form of the protobuf oneof
// constraints field in [ResourceAccessID]. Each variant maps to one of the
// concrete oneof wrapper types.
type constraintsJSON struct {
	UnknownConstraints *UnknownConstraints    `json:"unknown_constraints,omitempty"`
	AWSConsole         *AWSConsoleConstraints `json:"aws_console,omitempty"`
	SSH                *SSHConstraints        `json:"ssh,omitempty"`
}

// resourceAccessIDJSON mirrors [ResourceAccessID] with a concrete Constraints
// field so that encoding/json can marshal and unmarshal the value without
// needing to know the concrete interface type at runtime.
type resourceAccessIDJSON struct {
	Id          ResourceID       `json:"id"`
	Constraints *constraintsJSON `json:"constraints,omitempty"`
}

// MarshalJSON implements [json.Marshaler] for [ResourceAccessID].
func (r *ResourceAccessID) MarshalJSON() ([]byte, error) {
	out := resourceAccessIDJSON{Id: r.Id}
	switch c := r.Constraints.(type) {
	case *ResourceAccessID_UnknownConstraints:
		out.Constraints = &constraintsJSON{UnknownConstraints: c.UnknownConstraints}
	case *ResourceAccessID_AwsConsole:
		out.Constraints = &constraintsJSON{AWSConsole: c.AwsConsole}
	case *ResourceAccessID_Ssh:
		out.Constraints = &constraintsJSON{SSH: c.Ssh}
	}
	return json.Marshal(out)
}

// UnmarshalJSON implements [json.Unmarshaler] for [ResourceAccessID].
func (r *ResourceAccessID) UnmarshalJSON(data []byte) error {
	var tmp resourceAccessIDJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	r.Id = tmp.Id
	if tmp.Constraints == nil {
		return nil
	}
	switch {
	case tmp.Constraints.UnknownConstraints != nil:
		r.Constraints = &ResourceAccessID_UnknownConstraints{UnknownConstraints: tmp.Constraints.UnknownConstraints}
	case tmp.Constraints.AWSConsole != nil:
		r.Constraints = &ResourceAccessID_AwsConsole{AwsConsole: tmp.Constraints.AWSConsole}
	case tmp.Constraints.SSH != nil:
		r.Constraints = &ResourceAccessID_Ssh{Ssh: tmp.Constraints.SSH}
	}
	return nil
}
