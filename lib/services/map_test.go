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

package services

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

func TestRoleParsing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		roleMap types.RoleMap
		err     error
	}{
		{
			roleMap: nil,
		},
		{
			roleMap: types.RoleMap{
				{Remote: types.Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			roleMap: types.RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			roleMap: types.RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
		{
			roleMap: types.RoleMap{
				{Remote: types.Wildcard, Local: []string{"local-devs"}},
				{Remote: types.Wildcard, Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case '%v'", i), func(t *testing.T) {
			_, err := parseRoleMap(tc.roleMap)
			if tc.err != nil {
				require.Error(t, err)
				require.IsType(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRoleMap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		remote  []string
		local   []string
		roleMap types.RoleMap
		name    string
		err     error
	}{
		{
			name:    "all empty",
			remote:  nil,
			local:   nil,
			roleMap: nil,
		},
		{
			name:   "wildcard matches empty as well",
			remote: nil,
			local:  []string{"local-devs", "local-admins"},
			roleMap: types.RoleMap{
				{Remote: types.Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			name:   "direct match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: types.RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			name:   "direct match for multiple roles",
			remote: []string{"remote-devs", "remote-logs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: types.RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-logs", Local: []string{"local-logs"}},
			},
		},
		{
			name:   "direct match and wildcard",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: types.RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: types.Wildcard, Local: []string{"local-logs"}},
			},
		},
		{
			name:   "glob capture match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: types.RoleMap{
				{Remote: "remote-*", Local: []string{"local-$1"}},
			},
		},
		{
			name:   "passthrough match",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs"},
			roleMap: types.RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "passthrough match ignores implicit role",
			remote: []string{"remote-devs", constants.DefaultImplicitRole},
			local:  []string{"remote-devs"},
			roleMap: types.RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial match",
			remote: []string{"remote-devs", "something-else"},
			local:  []string{"remote-devs"},
			roleMap: types.RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial empty expand section is removed",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "remote-"},
			roleMap: types.RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1", "remote-$2", "$2"}},
			},
		},
		{
			name:   "multiple matches yield different results",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "test"},
			roleMap: types.RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
				{Remote: `^\Aremote-.*$`, Local: []string{"test"}},
			},
		},
		{
			name:   "mapping is deduplicated",
			remote: []string{"role1", "role2"},
			local:  []string{"foo"},
			roleMap: types.RoleMap{
				{Remote: "*", Local: []string{"foo"}},
			},
		},
		{
			name:   "different expand groups can be referred",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "devs"},
			roleMap: types.RoleMap{
				{Remote: "^(remote-(.*))$", Local: []string{"$1", "$2"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("test case '%v'", tc.name), func(t *testing.T) {

			local, err := MapRoles(tc.roleMap, tc.remote)
			if tc.err != nil {
				require.Error(t, err)
				require.IsType(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.local, local)
			}
		})
	}
}
