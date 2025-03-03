/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// Note: due to its complex interactions with Teleport, the `tctl terraform env`
// command is mainly not tested via unit tests but by integration tests validating the full flow.
// You can find its integration tests in `integration/tctl_terraform_env_test.go`

func TestTerraformCommand_checkIfRoleExists(t *testing.T) {
	// Test setup
	authHelper := integration.MinimalAuthHelper{}
	adminClient := authHelper.StartServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	newRoleFixture := func(name string) types.Role {
		role := services.NewPresetTerraformProviderRole()
		role.SetName(name)
		return role
	}

	tests := []struct {
		name string
		// Test setup
		existingRoleFlag string
		fixture          types.Role
		// Test validation
		expectedRoleName string
		expectedErr      require.ErrorAssertionFunc
	}{
		{
			name:             "Succeeds if preset role is found",
			existingRoleFlag: "",
			fixture:          newRoleFixture(teleport.PresetTerraformProviderRoleName),
			expectedRoleName: teleport.PresetTerraformProviderRoleName,
			expectedErr:      require.NoError,
		},
		{
			name:             "Fails if preset role is not found",
			existingRoleFlag: "",
			expectedRoleName: teleport.PresetTerraformProviderRoleName,
			expectedErr:      require.Error,
		},
		{
			name:             "Succeeds if custom existing role is specified and exists",
			existingRoleFlag: "existing-role",
			fixture:          newRoleFixture("existing-role"),
			expectedRoleName: "existing-role",
			expectedErr:      require.NoError,
		},
		{
			name:             "Fails if custom existing role is specified and does not exist",
			existingRoleFlag: "existing-role",
			expectedRoleName: "existing-role",
			expectedErr:      require.Error,
		},
	}
	for _, tt := range tests {
		// Warning: Those tests cannot be run in parallel
		t.Run(tt.name, func(t *testing.T) {
			// Test case setup
			if tt.fixture != nil {
				_, err := adminClient.CreateRole(ctx, tt.fixture)
				require.NoError(t, err)
			}

			// Test execution
			c := &TerraformCommand{
				existingRole: tt.existingRoleFlag,
				userOutput:   os.Stderr,
				log:          utils.NewSlogLoggerForTests(),
			}
			roleName, err := c.checkIfRoleExists(ctx, adminClient)
			tt.expectedErr(t, err)
			require.Equal(t, tt.expectedRoleName, roleName)

			// Test cleanup
			if tt.fixture != nil {
				require.NoError(t, adminClient.DeleteRole(ctx, roleName))
			}
		})
	}
}
