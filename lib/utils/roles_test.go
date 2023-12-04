/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParsing(t *testing.T) {
	t.Parallel()

	roles, err := types.ParseTeleportRoles("auth, Proxy,nODE")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(roles, types.SystemRoles{
		"Auth",
		"Proxy",
		"Node",
	}))

	require.NoError(t, roles[0].Check())
	require.NoError(t, roles[1].Check())
	require.NoError(t, roles[2].Check())
	require.NoError(t, roles.Check())
	require.Equal(t, "Auth,Proxy,Node", roles.String())
	require.Equal(t, "Auth", roles[0].String())
}

func TestBadRoles(t *testing.T) {
	t.Parallel()

	bad := types.SystemRole("bad-role")
	require.ErrorContains(t, bad.Check(), "role bad-role is not registered")
	badRoles := types.SystemRoles{
		bad,
		types.RoleAdmin,
	}
	require.ErrorContains(t, badRoles.Check(), "role bad-role is not registered")
}

func TestEquivalence(t *testing.T) {
	t.Parallel()

	nodeProxyRole := types.SystemRoles{
		types.RoleNode,
		types.RoleProxy,
	}
	authRole := types.SystemRoles{
		types.RoleAdmin,
		types.RoleAuth,
	}

	require.True(t, authRole.Include(types.RoleAdmin))
	require.False(t, authRole.Include(types.RoleProxy))
	require.False(t, authRole.Equals(nodeProxyRole))
	require.True(t, authRole.Equals(types.SystemRoles{types.RoleAuth, types.RoleAdmin}))
}
