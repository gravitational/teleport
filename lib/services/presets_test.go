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
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
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
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
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
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
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
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
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
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
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
							Roles:          []string{"some-role"},
							PreviewAsRoles: []string{"preview-role"},
						},
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
						ReviewRequests: &types.AccessReviewConditions{
							Roles: []string{
								teleport.PresetAccessRoleName,
								teleport.SystemIdentityCenterAccessRoleName,
								teleport.PresetGroupAccessRoleName,
								"some-role",
							},
							PreviewAsRoles: []string{
								teleport.PresetAccessRoleName,
								teleport.SystemIdentityCenterAccessRoleName,
								teleport.PresetGroupAccessRoleName,
								"preview-role",
							},
						},
					},
				},
			},
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
							Roles:         []string{"some-role"},
							SearchAsRoles: []string{"search-as-role"},
						},
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
						Request: &types.AccessRequestConditions{
							Roles: []string{"some-role"},
							SearchAsRoles: []string{
								teleport.PresetAccessRoleName,
								teleport.SystemIdentityCenterAccessRoleName,
								teleport.PresetGroupAccessRoleName,
								"search-as-role",
							},
						},
					},
				},
			},
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
						Request: &types.AccessRequestConditions{
							Roles: []string{"some-role"},
							SearchAsRoles: []string{
								teleport.SystemOktaAccessRoleName,
							},
						},
					},
				},
			},
		},
		{
			// This test is here to validate that we properly fix a bug previously introduced in the TF role preset.
			// All the new resources got added into the same rule, but the preset defaults system only supports adding
			// new rules, not editing existing ones. The resources got removed from the main rule and put into
			// smaller individual rules.
			name: "terraform provider (bugfix of the missing resources)",
			role: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetTerraformProviderRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default Terraform provider role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						AppLabels:            map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						DatabaseLabels:       map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						NodeLabels:           map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						WindowsDesktopLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						Rules: []types.Rule{
							{
								Resources: []string{
									types.KindAccessList,
									types.KindApp,
									types.KindClusterAuthPreference,
									types.KindClusterMaintenanceConfig,
									types.KindClusterNetworkingConfig,
									types.KindDatabase,
									types.KindDevice,
									types.KindGithub,
									types.KindLoginRule,
									types.KindNode,
									types.KindOIDC,
									types.KindOktaImportRule,
									types.KindRole,
									types.KindSAML,
									types.KindSessionRecordingConfig,
									types.KindToken,
									types.KindTrustedCluster,
									types.KindUser,
									// Some of the new resources got introduced, but not all
									types.KindBot,
									types.KindInstaller,
								},
								Verbs: RW(),
							},
						},
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetTerraformProviderRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default Terraform provider role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						AppLabels:            map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						DatabaseLabels:       map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						NodeLabels:           map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						WindowsDesktopLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
						Rules: []types.Rule{
							{
								Resources: []string{
									types.KindAccessList,
									types.KindApp,
									types.KindClusterAuthPreference,
									types.KindClusterMaintenanceConfig,
									types.KindClusterNetworkingConfig,
									types.KindDatabase,
									types.KindDevice,
									types.KindGithub,
									types.KindLoginRule,
									types.KindNode,
									types.KindOIDC,
									types.KindOktaImportRule,
									types.KindRole,
									types.KindSAML,
									types.KindSessionRecordingConfig,
									types.KindToken,
									types.KindTrustedCluster,
									types.KindUser,
									// The resources that already got into the main rule are still present.
									types.KindBot,
									types.KindInstaller,
								},
								Verbs: RW(),
							},
							// The missing resources got added as individual rules
							types.NewRule(types.KindAccessMonitoringRule, RW()),
							types.NewRule(types.KindDynamicWindowsDesktop, RW()),
							types.NewRule(types.KindStaticHostUser, RW()),
							types.NewRule(types.KindWorkloadIdentity, RW()),
							types.NewRule(types.KindGitServer, RW()),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.enterprise {
				modules.SetTestModules(t, &modules.TestModules{
					TestBuildType: modules.BuildEnterprise,
				})
			}

			role, err := AddRoleDefaults(context.Background(), test.role)
			test.expectedErr(t, err)

			require.Empty(t, cmp.Diff(role, test.expected))

			if test.expected != nil {
				require.Equal(t, test.reviewNotEmpty,
					!role.GetAccessReviewConditions(types.Allow).IsEmpty(),
					"Expected populated Access Review Conditions (%t)",
					test.reviewNotEmpty)

				require.Equal(t, test.accessRequestsNotEmpty,
					!role.GetAccessRequestConditions(types.Allow).IsEmpty(),
					"Expected populated Access Request Conditions (%t)",
					test.accessRequestsNotEmpty)
			}
		})
	}
}

func TestPresetRolesDumped(t *testing.T) {
	// This test ensures that the most recent version of selected preset roles
	// has been correctly dumped to a generated JSON file. We use a generated
	// file, because it's simpler to load it from a TypeScript test this way,
	// rather than calling a Go binary.

	// First, get the required roles, as defined in our codebase, and set their
	// defaults.
	access := NewPresetAccessRole()
	editor := NewPresetEditorRole()
	auditor := NewPresetAuditorRole()
	rolesByName := map[string]types.Role{
		access.GetName():  access,
		editor.GetName():  editor,
		auditor.GetName(): auditor,
	}
	for _, r := range rolesByName {
		err := CheckAndSetDefaults(r)
		require.NoError(t, err)
	}

	// Next, dump them all to JSON and parse them back again. This step is
	// necessary, because unmarshaling isn't precisely the opposite of
	// marshaling, and comparing raw roles to their unmarshaled counterparts
	// still lead to some discrepancies. We can't also directly compare JSON
	// blobs, as it's hard to reason whether this process is entirely
	// deterministic.
	bytes, err := json.Marshal(rolesByName)
	require.NoError(t, err)
	var recreatedRolesByName map[string]types.RoleV6
	err = json.Unmarshal(bytes, &recreatedRolesByName)
	require.NoError(t, err)

	// Read the roles defined in the generated file.
	bytes, err = os.ReadFile("../../gen/preset-roles.json")
	require.NoError(t, err)
	var rolesFromFile map[string]types.RoleV6
	err = json.Unmarshal(bytes, &rolesFromFile)
	require.NoError(t, err)

	// Finally, compare the roles.
	require.Equal(t, rolesFromFile, recreatedRolesByName,
		"The dumped preset roles differ from their representation in code. Please run:\n"+
			"make dump-preset-roles")
}
