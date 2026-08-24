// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decision

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

// MarshalSSHAccessPermit marshals an SSH access permit into its canonical JSON form.
func MarshalSSHAccessPermit(permit *decisionpb.SSHAccessPermit) (string, error) {
	permitJSON, err := protojson.Marshal(permit)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(permitJSON), nil
}

// UnmarshalSSHAccessPermit unmarshals an SSH access permit from its canonical JSON form.
func UnmarshalSSHAccessPermit(data string) (*decisionpb.SSHAccessPermit, error) {
	permit := &decisionpb.SSHAccessPermit{}

	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(data), permit); err != nil {
		return nil, trace.Wrap(err)
	}

	return permit, nil
}
