/*
Copyright 2015 Gravitational, Inc.

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
