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
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

func TestAddRoleDefaults(t *testing.T) {
	noChange := func(t require.TestingT, err error, i ...interface{}) {
		require.ErrorIs(t, err, trace.AlreadyExists("no change"))
	}
	notModifying := func(t require.TestingT, err error, i ...interface{}) {
		require.ErrorIs(t, err, trace.AlreadyExists("not modifying user created role"))
	}

	tests := []struct {
		name                    string
		role                    types.Role
		enterprise              bool
		reviewNotEmpty          bool
		accessRequestsNotEmpty  bool
		assumeRandomLoginsEqual bool

		expectedErr require.ErrorAssertionFunc
		expected    types.Role
	}{
		{
			name: "nothing added",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "editor",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetEditorRoleName],
					},
				},
			},
		},
		{
			name: "editor (only missing label)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetEditorRoleName],
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetEditorRoleName],
					},
				},
			},
		},
		{
			name: "access (access review, db labels, identical rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
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
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
		},
		{
			name: "access (only missing label)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels()[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
		},
		{
			name: "auditor",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						RecordSession: &types.RecordSession{
							Desktop: types.NewBoolOption(false),
						},
					},
				},
			},
			assumeRandomLoginsEqual: true,
			expectedErr:             require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						RecordSession: &types.RecordSession{
							Desktop: types.NewBoolOption(false),
						},
					},
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetAuditorRoleName],
					},
				},
			},
		},
		{
			name: "auditor (only missing label)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						RecordSession: &types.RecordSession{
							Desktop: types.NewBoolOption(false),
						},
					},
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetAuditorRoleName],
					},
				},
			},
			assumeRandomLoginsEqual: true,
			expectedErr:             require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						RecordSession: &types.RecordSession{
							Desktop: types.NewBoolOption(false),
						},
					},
					Allow: types.RoleConditions{
						Rules: defaultAllowRules()[teleport.PresetAuditorRoleName],
					},
				},
			},
		},
		{
			name: "reviewer (not enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetReviewerRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "reviewer (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetReviewerRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
			},
			enterprise:     true,
			expectedErr:    require.NoError,
			reviewNotEmpty: true,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetReviewerRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						ReviewRequests: defaultAllowAccessReviewConditions(true)[teleport.PresetReviewerRoleName],
					},
				},
			},
		},
		{
			name: "reviewer (enterprise, created by user)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetReviewerRoleName,
				},
			},
			enterprise:  true,
			expectedErr: notModifying,
			expected:    nil,
		},
		{
			name: "reviewer (enterprise, existing review requests)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetReviewerRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{"some-role"},
						},
					},
				},
			},
			enterprise:  true,
			expectedErr: noChange,
		},
		{
			name: "requester (not enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetRequesterRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: types.IsManaged,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "requester (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetRequesterRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
			},
			enterprise:             true,
			expectedErr:            require.NoError,
			accessRequestsNotEmpty: true,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetRequesterRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Request: defaultAllowAccessRequestConditions(true)[teleport.PresetRequesterRoleName],
					},
				},
			},
		},
		{
			name: "requester (enterprise, created by user)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetRequesterRoleName,
				},
			},
			enterprise:  true,
			expectedErr: notModifying,
			expected:    nil,
		},
		{
			name: "requester (enterprise, existing requests)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetRequesterRoleName,
					Labels: map[string]string{
						types.TeleportManagedLabel: "true",
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Request: &types.AccessRequestConditions{
							Roles: []string{"some-role"},
						},
					},
				},
			},
			enterprise:  true,
			expectedErr: noChange,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.enterprise {
				modules.SetTestModules(t, &modules.TestModules{
					TestBuildType: modules.BuildEnterprise,
				})
			}

			role, err := AddRoleDefaults(test.role)
			test.expectedErr(t, err)

			if test.assumeRandomLoginsEqual && test.expected != nil && role != nil {
				test.expected.SetLogins(types.Allow, role.GetLogins(types.Allow))
			}

			require.Empty(t, cmp.Diff(role, test.expected))

			if test.expected != nil {
				require.Equal(t, test.reviewNotEmpty, !role.GetAccessReviewConditions(types.Allow).IsEmpty())
				require.Equal(t, test.accessRequestsNotEmpty, !role.GetAccessRequestConditions(types.Allow).IsEmpty())
			}
		})
	}
}
