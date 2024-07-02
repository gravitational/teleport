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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
)

// Note: due to its complex interactions with Teleport, the `tctl terraform env`
// command is mainly not tested via unit tests but by integration tests validating the full flow.
// You can find its integration tests in `integration/tctl_terraform_env_test.go`

func TestTerraformCommand_createRoleIfNeeded(t *testing.T) {
	// Test setup
	authHelper := integration.MinimalAuthHelper{}
	adminClient := authHelper.StartServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Setting labels allows us to check whether the role
	// has been updated by the helper or not.
	defaultLabels := terraformRoleSpec.Allow.AppLabels
	testLabels := types.Labels{"foo": []string{"bar"}}
	existingRoleSpec := terraformRoleSpec
	existingRoleSpec.Allow.AppLabels = testLabels

	newRoleFixture := func(t *testing.T, name string) types.Role {
		role, err := types.NewRole(name, existingRoleSpec)
		require.NoError(t, err)
		return role
	}

	tests := []struct {
		name string
		// Test setup
		resourcePrefixFlag string
		existingRoleFlag   string
		fixture            types.Role
		// Test validation
		wantErr               require.ErrorAssertionFunc
		expectedRoleName      string
		expectedRoleAppLabels types.Labels
	}{
		{
			name:                  "Create role when not exist",
			wantErr:               require.NoError,
			expectedRoleAppLabels: defaultLabels,
			expectedRoleName:      terraformHelperDefaultResourcePrefix + "provider",
		},
		{
			name:                  "Update existing role",
			fixture:               newRoleFixture(t, terraformHelperDefaultResourcePrefix+"provider"),
			wantErr:               require.NoError,
			expectedRoleAppLabels: defaultLabels,
			expectedRoleName:      terraformHelperDefaultResourcePrefix + "provider",
		},
		{
			name:                  "Honour resource prefix",
			resourcePrefixFlag:    "test-",
			wantErr:               require.NoError,
			expectedRoleName:      "test-provider",
			expectedRoleAppLabels: defaultLabels,
		},
		{
			name:                  "Does not change existing role",
			existingRoleFlag:      "existing-role",
			fixture:               newRoleFixture(t, "existing-role"),
			wantErr:               require.NoError,
			expectedRoleName:      "existing-role",
			expectedRoleAppLabels: testLabels,
		},
		{
			name:             "Fails if existing role is not found",
			existingRoleFlag: "existing-role",
			wantErr:          require.Error,
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
			// mimick the kingpin default behaviour
			resourcePrefix := tt.resourcePrefixFlag
			if resourcePrefix == "" {
				resourcePrefix = terraformHelperDefaultResourcePrefix
			}

			// Test execution
			c := &TerraformCommand{
				resourcePrefix: resourcePrefix,
				existingRole:   tt.existingRoleFlag,
				userOutput:     os.Stderr,
			}
			roleName, err := c.createRoleIfNeeded(ctx, adminClient)
			tt.wantErr(t, err)
			require.Equal(t, tt.expectedRoleName, roleName)
			if tt.expectedRoleAppLabels != nil {
				gotRole, err := adminClient.GetRole(ctx, roleName)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(tt.expectedRoleAppLabels, gotRole.GetAppLabels(types.Allow)))
			}

			// Test cleanup
			if roleName != "" {
				err = adminClient.DeleteRole(ctx, roleName)
				if !trace.IsNotFound(err) {
					require.NoError(t, err)
				}
			}
		})
	}
}
