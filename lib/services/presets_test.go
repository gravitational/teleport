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
		name                   string
		role                   types.Role
		enterprise             bool
		reviewNotEmpty         bool
		accessRequestsNotEmpty bool

		expectedErr require.ErrorAssertionFunc
		expected    types.Role
	}{
		{
			name: "nothing added",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "editor (default rules match preset rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetEditorRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: NewPresetEditorRole().GetRules(types.Allow),
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
						types.TeleportInternalResourceType: types.PresetResource,
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
			name: "access (default rules match preset rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 NewPresetAccessRole().GetRules(types.Allow),
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseRoles,
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
						DatabaseServiceLabels: defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseServiceLabels: defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseServiceLabels,
						DatabaseRoles:         defaultAllowLabels(false)[teleport.PresetAccessRoleName].DatabaseRoles,
						Rules:                 defaultAllowRules()[teleport.PresetAccessRoleName],
					},
				},
			},
		},
		{
			name: "auditor (default rules match preset rules)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
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
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
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
						Rules: NewPresetAuditorRole().GetRules(types.Allow),
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
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetAuditorRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
						types.TeleportInternalResourceType: types.PresetResource,
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
		{
			name: "okta resources (not enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "okta resources (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
					},
				},
			},
			enterprise:  true,
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaAccessRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						AppLabels: types.Labels{
							types.OriginLabel: []string{types.OriginOkta},
						},
						GroupLabels: types.Labels{
							types.OriginLabel: []string{types.OriginOkta},
						},
					},
				},
			},
		},
		{
			name: "okta resources (enterprise, created by user)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaAccessRoleName,
				},
			},
			enterprise:  true,
			expectedErr: notModifying,
			expected:    nil,
		},
		{
			name: "okta requester (not enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaRequesterRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
						types.OriginLabel:                  types.OriginOkta,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
		{
			name: "okta requester (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaRequesterRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
						types.OriginLabel:                  types.OriginOkta,
					},
				},
			},
			enterprise:             true,
			expectedErr:            require.NoError,
			accessRequestsNotEmpty: true,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaRequesterRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
						types.OriginLabel:                  types.OriginOkta,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Request: defaultAllowAccessRequestConditions(true)[teleport.SystemOktaRequesterRoleName],
					},
				},
			},
		},
		{
			name: "okta requester (enterprise, created by user)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaRequesterRoleName,
				},
			},
			enterprise:  true,
			expectedErr: notModifying,
			expected:    nil,
		},
		{
			name: "okta requester (enterprise, existing requests)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.SystemOktaRequesterRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.SystemResource,
						types.OriginLabel:                  types.OriginOkta,
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

			require.Empty(t, cmp.Diff(role, test.expected))

			if test.expected != nil {
				require.Equal(t, test.reviewNotEmpty, !role.GetAccessReviewConditions(types.Allow).IsEmpty())
				require.Equal(t, test.accessRequestsNotEmpty, !role.GetAccessRequestConditions(types.Allow).IsEmpty())
			}
		})
	}
}
