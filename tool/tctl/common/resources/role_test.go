/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func roleWithAppResources(allow, deny []types.AppResource) types.Role {
	return &types.RoleV6{
		Metadata: types.Metadata{Name: "test-role"},
		Version:  types.V9,
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{AppResources: allow},
			Deny:  types.RoleConditions{AppResources: deny},
		},
	}
}

// unknownField sets field 10, which no version declares.
var unknownField = types.AppResource{AllowAll: true, XXX_unrecognized: []byte{0x50, 0x01}}

func TestCheckUnknownAppResourcesFields(t *testing.T) {
	known := types.AppResource{AllowAll: true}

	require.NoError(t, checkUnknownAppResourcesFields(roleWithAppResources(nil, nil)))
	require.NoError(t, checkUnknownAppResourcesFields(roleWithAppResources([]types.AppResource{known}, nil)))

	err := checkUnknownAppResourcesFields(roleWithAppResources([]types.AppResource{unknownField}, nil))
	require.ErrorContains(t, err, "does not recognize")

	err = checkUnknownAppResourcesFields(roleWithAppResources(nil, []types.AppResource{unknownField}))
	require.ErrorContains(t, err, "does not recognize")
}

func TestWarnAboutUnknownAppResourcesFields(t *testing.T) {
	var buf strings.Builder
	warnAboutUnknownAppResourcesFields(&buf, roleWithAppResources([]types.AppResource{{AllowAll: true}}, nil))
	require.Empty(t, buf.String())

	warnAboutUnknownAppResourcesFields(&buf, roleWithAppResources([]types.AppResource{unknownField}, nil))
	require.Contains(t, buf.String(), "does not recognize")
	require.Contains(t, buf.String(), "test-role")
}
