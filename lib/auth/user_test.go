/*
Copyright 2022 Gravitational, Inc.

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

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestUserConstraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	testRoles := []struct {
		Name  string
		Rules []types.Rule
	}{
		{
			Name: "test-role-1",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindRole},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
		{
			Name: "test-role-2",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate, types.VerbUpdate},
				},
			},
		},
	}

	// create and insert roles
	for _, r := range testRoles {
		role, err := types.NewRoleV3(r.Name, types.RoleSpecV5{
			Options: types.RoleOptions{},
			Allow: types.RoleConditions{
				Rules: r.Rules,
			},
		})
		require.NoError(t, err)

		err = p.a.UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	// create and insert roles
	user1, err := types.NewUser("user-1")
	require.NoError(t, err)
	user1.SetRoles([]string{testRoles[0].Name})

	err = p.a.CreateUser(ctx, user1)
	require.NoError(t, err)

	user2, err := types.NewUser("user-2")
	require.NoError(t, err)
	user2.SetRoles([]string{testRoles[0].Name})

	err = p.a.CreateUser(ctx, user2)
	require.NoError(t, err)

	user3, err := types.NewUser("user-3")
	require.NoError(t, err)
	user2.SetRoles([]string{testRoles[1].Name})

	err = p.a.CreateUser(ctx, user3)
	require.NoError(t, err)

	// remove test-role-1 role from user-2.
	// the operation will succeed because user-1 still has the permission
	// to upsert roles.
	user2.SetRoles([]string{})
	err = p.a.UpdateUser(ctx, user2)
	require.NoError(t, err)

	// remove test-role-1 role from user-1.
	// the operation will fail because user-1 is the only user left with the
	// permission to upsert roles.
	user1.SetRoles([]string{})
	err = p.a.UpdateUser(ctx, user1)
	require.Error(t, err)

	// remove test-role-2 user-3.
	// this is a control operation that shows that other users
	// are not affected by the "role resource constraints".
	user3.SetRoles([]string{})
	err = p.a.UpdateUser(ctx, user3)
	require.NoError(t, err)

	// delete user-1.
	// the operation will fail because user-1 is the only user left with the
	// permission to upsert roles.
	err = p.a.DeleteUser(ctx, user1.GetName())
	require.Error(t, err)

	// delete user-3.
	// this is a control operation that shows that other users
	// are not affected by the "role resource constraints".
	err = p.a.DeleteUser(ctx, user3.GetName())
	require.NoError(t, err)
}
