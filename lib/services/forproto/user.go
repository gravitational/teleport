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

	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/forproto/internal"
)

// User is a potentially pre-marshaled [*types.UserV2].
type User struct {
	internal.Resource[types.UserV2, UserExtra, userDriver]
}

// the unused linter doesn't see through internal.Resource
var _ userDriver

type UserExtra struct {
	// Name is the name of the role.
	Name string
}

type userDriver struct {
	internal.GogoMessageDriver[*types.UserV2, types.UserV2]
}

// GetExtra implements [internal.Driver].
func (userDriver) GetExtra(actual *types.UserV2) UserExtra {
	return UserExtra{
		Name: actual.Metadata.Name,
	}
}

// ListUsesFieldOne implements [internal.DriverFieldOne].
func (userDriver) ListUsesFieldOne() {}

// ToListUsersResponse turns the response from ListUsersForProto into a message
// suitable for returning from gRPC.
func ToListUsersResponse(resources []User, nextPageToken string, err error) (*usersv1.ListUsersResponse, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actual, wire, err := internal.ResourcesToProtoFieldOne(resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := usersv1.ListUsersResponse_builder{
		Users:         actual,
		NextPageToken: nextPageToken,
	}.Build()
	resp.ProtoReflect().SetUnknown(wire)

	return resp, nil
}

// FromListUsers converts a ListUsers response into the response expected from
// ListUsersForProto.
func FromListUsers(resp *usersv1.ListUsersResponse, err error) (_ []User, nextPageToken string, _ error) {
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return internal.ResourcesFromActual[User](resp.GetUsers()), resp.GetNextPageToken(), nil
}
