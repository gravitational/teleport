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

package types

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility
	"github.com/gravitational/trace"
)

const (
	ResourceConstraintVersionV1 = V1
)

func (rc *ResourceConstraints) CheckAndSetDefaults() error {
	if rc.Version == "" {
		rc.Version = ResourceConstraintVersionV1
	} else if rc.Version != ResourceConstraintVersionV1 {
		return trace.BadParameter("unsupported Constraints version %q", rc.Version)
	}
	switch d := rc.Details.(type) {
	case *ResourceConstraints_AwsConsole:
		if err := d.Validate(); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported Details type %T", d)
	}
	return nil
}

func (rc *ResourceConstraints) MarshalJSON() ([]byte, error) {
	if rc == nil {
		return []byte("undefined"), nil
	}
	var buf bytes.Buffer
	m := &jsonpb.Marshaler{
		OrigName:     true,
		EnumsAsInts:  true,
		EmitDefaults: false,
	}
	if err := m.Marshal(&buf, rc); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func (rc *ResourceConstraints) UnmarshalJSON(b []byte) error {
	u := &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
	}
	return trace.Wrap(u.Unmarshal(bytes.NewReader(b), rc))
}

// Validate ensures RoleArns is non-nil and contains Role ARNs.
func (awsc *ResourceConstraints_AwsConsole) Validate() error {
	if awsc == nil || awsc.AwsConsole == nil || len(awsc.AwsConsole.RoleArns) == 0 {
		return trace.BadParameter("aws_console constraints require role_arns, none provided")
	}
	return nil
}
