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

package forproto

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/forproto/internal"
)

func roleHasGranularSSHPortForwarding(role *types.RoleV6) bool {
	// should be kept in sync with the downgrade logic in lib/auth/grpcserver.go
	sshPortForwarding := role.Spec.Options.SSHPortForwarding
	return sshPortForwarding != nil && (sshPortForwarding.Remote != nil || sshPortForwarding.Local != nil)
}

// Role is a potentially pre-marshaled [*types.RoleV6].
type Role struct {
	internal.Resource[types.RoleV6, RoleExtra, roleDriver]
}

// the unused linter doesn't see through internal.Resource
var _ roleDriver

type RoleExtra struct {
	// Version is the role version.
	Version string

	// GranularSSHPortForwarding is set if the role contains granular SSH port
	// forwarding rules (i.e. potentially different rules for local and remote).
	GranularSSHPortForwarding bool
}

type roleDriver struct {
	internal.GogoMessageDriver[*types.RoleV6, types.RoleV6]
}

// GetExtra implements [internal.Driver].
func (roleDriver) GetExtra(actual *types.RoleV6) RoleExtra {
	return RoleExtra{
		Version:                   actual.Version,
		GranularSSHPortForwarding: roleHasGranularSSHPortForwarding(actual),
	}
}

// ListUsesFieldOne implements [internal.DriverFieldOne].
func (roleDriver) ListUsesFieldOne() {}

// ToListRolesResponse turns the response from ListRolesForProto into a message
// suitable for returning from gRPC.
func ToListRolesResponse(resources []Role, nextPageToken string, err error) (*proto.ListRolesResponse, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actual, wire, err := internal.ResourcesToProtoFieldOne(resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.ListRolesResponse{
		Roles:            actual,
		NextKey:          nextPageToken,
		XXX_unrecognized: wire,
	}, nil
}

// FromListRoles converts a ListRoles response into the response expected from
// ListRolesForProto.
func FromListRoles(resp *proto.ListRolesResponse, err error) (_ []Role, nextPageToken string, _ error) {
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return internal.ResourcesFromActual[Role](resp.GetRoles()), resp.GetNextKey(), nil
}
