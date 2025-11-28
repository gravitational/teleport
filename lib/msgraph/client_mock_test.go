// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package msgraph

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestClientMockIterators(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultMockState := NewMockedMSGraphState()
	var c = NewClientMock(&defaultMockState)

	t.Run("should list users", func(t *testing.T) {
		out := make([]*User, 0, len(defaultMockState.Users))
		err := c.IterateUsers(ctx, func(u *User) bool {
			out = append(out, u)
			return true
		})
		require.NoError(t, err)

		require.ElementsMatch(t, defaultMockState.Users, out)
	})

	t.Run("should list groups", func(t *testing.T) {
		out := make([]*Group, 0, len(defaultMockState.Groups))
		err := c.IterateGroups(ctx, func(g *Group) bool {
			out = append(out, g)
			return true
		})
		require.NoError(t, err)

		require.ElementsMatch(t, defaultMockState.Groups, out)
	})

	t.Run("should list group members", func(t *testing.T) {
		out := make(map[string][]GroupMember)
		groups := []*Group{}
		err := c.IterateGroups(ctx, func(g *Group) bool {
			groups = append(groups, g)
			return true
		})
		require.NoError(t, err)

		for _, g := range groups {
			var members []GroupMember
			err = c.IterateGroupMembers(ctx, *g.GetID(), func(u GroupMember) bool {
				members = append(members, u)
				return true
			})
			require.NoError(t, err)
			out[*g.GetID()] = members
		}

		require.Equal(t, defaultMockState.GroupMembers, out)
	})

	t.Run("should list applications", func(t *testing.T) {
		out := make([]*Application, 0, len(defaultMockState.Applications))
		err := c.IterateApplications(ctx, func(a *Application) bool {
			out = append(out, a)
			return true
		})
		require.NoError(t, err)

		require.ElementsMatch(t, defaultMockState.Applications, out)
	})

	t.Run("should get application", func(t *testing.T) {
		app, err := c.GetApplication(ctx, "app1")
		require.NoError(t, err)

		require.NotNil(t, app)
	})
}

func TestMonkeyPatch(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultMockState := NewMockedMSGraphState()
	var c = NewClientMock(&defaultMockState)

	t.Run("patch IterateUsers to return an error", func(t *testing.T) {
		out := make([]*User, 0, len(defaultMockState.Users))
		c.MonkeyPatch.IterateUsers = func(ctx context.Context, f func(u *User) bool, opts ...IterateOpt) error {
			return trace.Errorf("error fetching users")
		}
		err := c.IterateUsers(ctx, func(u *User) bool {
			out = append(out, u)
			return true
		})
		require.ErrorContains(t, err, "error fetching users")
	})
}
