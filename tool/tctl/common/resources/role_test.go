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

func roleWithAppResources(t *testing.T, allow, deny []types.AppResource) types.Role {
	t.Helper()
	role, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err)
	roleV6, ok := role.(*types.RoleV6)
	require.True(t, ok)
	roleV6.Spec.Allow.AppResources = allow
	roleV6.Spec.Deny.AppResources = deny
	return roleV6
}

func TestCheckUnknownAppResourcesFields(t *testing.T) {
	unknown := types.AppResource{AllowAll: true, XXX_unrecognized: []byte{0x0a, 0x01, 0x2f}}
	known := types.AppResource{AllowAll: true}

	require.NoError(t, checkUnknownAppResourcesFields(roleWithAppResources(t, nil, nil)))
	require.NoError(t, checkUnknownAppResourcesFields(roleWithAppResources(t, []types.AppResource{known}, nil)))

	err := checkUnknownAppResourcesFields(roleWithAppResources(t, []types.AppResource{unknown}, nil))
	require.ErrorContains(t, err, "does not recognize")

	// A newer auth server may permit deny rules, which drop just as silently.
	err = checkUnknownAppResourcesFields(roleWithAppResources(t, nil, []types.AppResource{unknown}))
	require.ErrorContains(t, err, "does not recognize")
}

func TestWarnAboutUnknownAppResourcesFields(t *testing.T) {
	unknown := types.AppResource{AllowAll: true, XXX_unrecognized: []byte{0x0a, 0x01, 0x2f}}

	var buf strings.Builder
	warnAboutUnknownAppResourcesFields(&buf, roleWithAppResources(t, []types.AppResource{{AllowAll: true}}, nil))
	require.Empty(t, buf.String())

	warnAboutUnknownAppResourcesFields(&buf, roleWithAppResources(t, []types.AppResource{unknown}, nil))
	require.Contains(t, buf.String(), "does not recognize")
	require.Contains(t, buf.String(), "test-role")
}
