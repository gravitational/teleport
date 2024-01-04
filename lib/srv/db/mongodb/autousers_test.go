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

package mongodb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_makeUserRoles(t *testing.T) {
	tests := []struct {
		name            string
		input           []string
		checkError      require.ErrorAssertionFunc
		expectUserRoles userRoles
	}{
		{
			name: "input is invalid",
			input: []string{
				"read@admin",
				"permissionWithoutPermissionDefined",
			},
			checkError: require.Error,
		},
		{
			name: "output is sorted",
			input: []string{
				"write@test",
				"my-custom-role@test2",
				"readAnyDatabase@admin",
				"read@test",
			},
			checkError: require.NoError,
			expectUserRoles: userRoles{
				// Sort by Database first, then Rolename.
				{Database: "admin", Rolename: "readAnyDatabase"},
				{Database: "test", Rolename: "read"},
				{Database: "test", Rolename: "write"},
				{Database: "test2", Rolename: "my-custom-role"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualUserRoles, err := makeUserRoles(test.input)
			test.checkError(t, err)
			require.Equal(t, test.expectUserRoles, actualUserRoles)
		})
	}
}
