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
	"encoding/json"

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
	case *ResourceConstraints_Ssh:
		if err := d.Validate(); err != nil {
			return trace.Wrap(err)
		}
	case nil:
		return trace.BadParameter("constraints carry no supported content; they are either empty or from a newer Teleport version")
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
	strict := &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
	}
	if err := strict.Unmarshal(bytes.NewReader(b), rc); err != nil {
		// Content that doesn't strictly decode: an unknown kind, or an
		// unknown field in a known one.
		return trace.Wrap(rc.unmarshalVersionOnly(b, err))
	}
	if rc.Version != "" && rc.Version != ResourceConstraintVersionV1 {
		// Fields all decoded, but the version is newer than this build
		// understands. Keep only the version; the entry becomes
		// unenforceable.
		*rc = ResourceConstraints{Version: rc.Version}
	}
	return nil
}

// unmarshalVersionOnly resets rc to carry only the version found in b,
// leaving it unenforceable for enforcement to deny. If b yields no version
// at all, rc is zeroed and strictErr is returned so the caller doesn't use
// a half-parsed value.
func (rc *ResourceConstraints) unmarshalVersionOnly(b []byte, strictErr error) error {
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		*rc = ResourceConstraints{}
		return strictErr
	}
	*rc = ResourceConstraints{Version: v.Version}
	return nil
}

// Unenforceable reports whether the constraints carry content this build
// cannot decode or validate. Such entries must deny their resource at
// enforcement rather than being treated as unconstrained.
func (rc *ResourceConstraints) Unenforceable() bool {
	return rc != nil && rc.Details == nil
}

// Validate ensures RoleArns is non-nil and contains Role ARNs.
func (awsc *ResourceConstraints_AwsConsole) Validate() error {
	if awsc == nil || awsc.AwsConsole == nil || len(awsc.AwsConsole.RoleArns) == 0 {
		return trace.BadParameter("aws_console constraints require role_arns, none provided")
	}
	return nil
}

// Validate ensures Logins is non-nil and contains logins.
func (sshc *ResourceConstraints_Ssh) Validate() error {
	if sshc == nil || sshc.Ssh == nil || len(sshc.Ssh.Logins) == 0 {
		return trace.BadParameter("ssh constraints require logins, none provided")
	}
	return nil
}
