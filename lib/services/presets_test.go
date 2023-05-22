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

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

func TestAddRoleDefaults(t *testing.T) {
	noChange := func(t require.TestingT, err error, i ...interface{}) {
		require.ErrorIs(t, err, trace.AlreadyExists("no change"))
	}

	tests := []struct {
		name        string
		role        types.Role
		expectedErr require.ErrorAssertionFunc
		expected    types.Role
	}{
		{
			name:        "nothing added",
			role:        &types.RoleV6{},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "editor",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules:          defaultAllowRules()[teleport.PresetEditorRoleName],
						ReviewRequests: defaultAllowAccessReviewConditions()[teleport.PresetEditorRoleName],
					},
				},
			},
		},
		{
			name: "editor (existing rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							{
								Resources: []string{"test"},
								Verbs:     []string{"test"},
							},
						},
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: append([]types.Rule{
							{
								Resources: []string{"test"},
								Verbs:     []string{"test"},
							},
						}, defaultAllowRules()[teleport.PresetEditorRoleName]...),
						ReviewRequests: defaultAllowAccessReviewConditions()[teleport.PresetEditorRoleName],
					},
				},
			},
		},
		{
			name: "editor (existing review requests, identical rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetEditorRoleName],
						ReviewRequests: &types.AccessReviewConditions{
							Where: "test",
						},
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "access (access review, db labels, identical rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
						Request:               defaultAllowAccessRequestConditions()[teleport.PresetAccessRoleName],
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role, err := AddRoleDefaults(test.role)
			test.expectedErr(t, err)

			require.Empty(t, cmp.Diff(role, test.expected))
		})
	}
}
