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

package ui

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestNewUserContext(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
		Status: types.UserStatusV2{
			PasswordState: types.PasswordState_PASSWORD_STATE_SET,
		},
	}

	// set some rules
	role1 := &types.RoleV6{}
	role1.SetNamespaces(types.Allow, []string{apidefaults.Namespace})

	role2 := &types.RoleV6{}
	role2.SetNamespaces(types.Allow, []string{apidefaults.Namespace})

	roleSet := []types.Role{role1, role2}
	userContext, err := NewUserContext(user, roleSet, proto.Features{}, true, false)
	require.NoError(t, err)

	// test user name
	require.Equal(t, "root", userContext.Name)
	require.Empty(t, cmp.Diff(userContext.AccessStrategy, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	}))
	require.Equal(t, types.PasswordState_PASSWORD_STATE_SET, userContext.PasswordSate)

	// test local auth type
	require.Equal(t, authLocal, userContext.AuthType)

	// test sso auth type
	user.Spec.GithubIdentities = []types.ExternalIdentity{{ConnectorID: "foo", Username: "bar"}}
	userContext, err = NewUserContext(user, roleSet, proto.Features{}, true, false)
	require.NoError(t, err)
	require.Equal(t, authSSO, userContext.AuthType)
}

func TestNewUserContextCloud(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
	}

	role := &types.RoleV6{}
	role.SetNamespaces(types.Allow, []string{"*"})

	roleSet := []types.Role{role}

	userContext, err := NewUserContext(user, roleSet, proto.Features{Cloud: true}, true, false)
	require.NoError(t, err)

	require.Equal(t, "root", userContext.Name)
	require.Empty(t, cmp.Diff(userContext.AccessStrategy, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	}))
}
