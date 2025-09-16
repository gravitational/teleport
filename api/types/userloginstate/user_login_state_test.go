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

package userloginstate

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
)

func TestIsOriginalRolesAndTraitsSet(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "not set",
			labels:   map[string]string{},
			expected: false,
		},
		{
			name: "set to true",
			labels: map[string]string{
				OriginalRolesAndTraitsSet: "true",
			},
			expected: true,
		},
		{
			name: "set to an empty value",
			labels: map[string]string{
				OriginalRolesAndTraitsSet: "",
			},
			expected: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			uls, err := New(
				header.Metadata{
					Name:   "uls",
					Labels: test.labels,
				},
				Spec{
					OriginalRoles: []string{"role1"},
					OriginalTraits: trait.Traits{
						"key1": []string{"value1"},
					},
					Roles: []string{"role1", "role2"},
					Traits: trait.Traits{
						"key1": []string{"value1"},
						"key2": []string{"value2"},
					},
					UserType: types.UserTypeSSO,
				},
			)
			require.NoError(t, err)

			require.Equal(t, test.expected, uls.IsOriginalRolesAndTraitsSet())
		})
	}
}
