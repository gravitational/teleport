/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestAccessRequestCreateStructuredOutput verifies that `tctl requests create`
// keeps printing the request name by default and serializes the created access
// request resource when --format=json or --format=yaml is requested.
func TestAccessRequestCreateStructuredOutput(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

	// A user that is allowed to request the preset "access" role.
	role, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{teleport.PresetAccessRoleName},
			},
		},
	})
	require.NoError(t, err)
	_, err = clt.CreateRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser("requester-user")
	require.NoError(t, err)
	user.SetRoles([]string{role.GetName()})
	_, err = clt.CreateUser(ctx, user)
	require.NoError(t, err)

	t.Run("text prints request name", func(t *testing.T) {
		buf, err := runRequestCommand(t, clt, []string{
			"create", user.GetName(), "--roles=" + teleport.PresetAccessRoleName,
		})
		require.NoError(t, err)
		// Default text output is a bare request ID followed by a newline.
		require.NotEmpty(t, buf.String())
		require.NotContains(t, buf.String(), "{")
	})

	t.Run("json", func(t *testing.T) {
		buf, err := runRequestCommand(t, clt, []string{
			"create", user.GetName(), "--roles=" + teleport.PresetAccessRoleName,
			"--format=json",
		})
		require.NoError(t, err)

		got := mustDecodeJSON[*types.AccessRequestV3](t, buf)
		require.NotEmpty(t, got.GetName())
		require.Equal(t, user.GetName(), got.GetUser())
		require.Equal(t, []string{teleport.PresetAccessRoleName}, got.GetRoles())
	})

	t.Run("yaml", func(t *testing.T) {
		buf, err := runRequestCommand(t, clt, []string{
			"create", user.GetName(), "--roles=" + teleport.PresetAccessRoleName,
			"--format=yaml",
		})
		require.NoError(t, err)

		got := mustDecodeJSON[*types.AccessRequestV3](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, buf)))
		require.NotEmpty(t, got.GetName())
		require.Equal(t, user.GetName(), got.GetUser())
		require.Equal(t, []string{teleport.PresetAccessRoleName}, got.GetRoles())
	})
}
