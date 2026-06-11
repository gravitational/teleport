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

package auth

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMaybeDowngradeRoleCreateDatabaseUserMode(t *testing.T) {
	fullMajorBehind := semver.Version{Major: 17, Minor: 0, Patch: 0}
	oneMinorBehind := semver.Version{Major: 18, Minor: 10, Patch: 0}
	atIntroducingVersion := minSupportedCreateDatabaseUserReassignAndDropVersion
	newerClient := semver.Version{Major: 19, Minor: 0, Patch: 0}

	roleWithMode := func(mode types.CreateDatabaseUserMode) *types.RoleV6 {
		return &types.RoleV6{
			Metadata: types.Metadata{Name: "test"},
			Spec: types.RoleSpecV6{
				Options: types.RoleOptions{CreateDatabaseUserMode: mode},
			},
		}
	}

	tests := []struct {
		name           string
		clientVersion  semver.Version
		inputMode      types.CreateDatabaseUserMode
		wantMode       types.CreateDatabaseUserMode
		wantDowngraded bool
	}{
		{
			name:           "client a full major behind is downgraded",
			clientVersion:  fullMajorBehind,
			inputMode:      types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantMode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			wantDowngraded: true,
		},
		{
			name:           "client one minor behind is downgraded",
			clientVersion:  oneMinorBehind,
			inputMode:      types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantMode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			wantDowngraded: true,
		},
		{
			name:           "client at introducing version keeps reassign_and_drop",
			clientVersion:  atIntroducingVersion,
			inputMode:      types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantMode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantDowngraded: false,
		},
		{
			name:           "newer client keeps reassign_and_drop",
			clientVersion:  newerClient,
			inputMode:      types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantMode:       types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP,
			wantDowngraded: false,
		},
		{
			name:           "older client leaves other modes untouched",
			clientVersion:  oneMinorBehind,
			inputMode:      types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			wantMode:       types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			wantDowngraded: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientVersion := tc.clientVersion
			got := maybeDowngradeRoleCreateDatabaseUserMode(roleWithMode(tc.inputMode), &clientVersion)

			require.Equal(t, tc.wantMode, got.GetOptions().CreateDatabaseUserMode)
			_, downgraded := got.GetMetadata().Labels[types.TeleportDowngradedLabel]
			require.Equal(t, tc.wantDowngraded, downgraded)
		})
	}
}
