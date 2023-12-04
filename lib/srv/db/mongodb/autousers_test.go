/*
Copyright 2023 Gravitational, Inc.

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
