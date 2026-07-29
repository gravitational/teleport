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
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAddRoleDefaults(t *testing.T) {
	noChange := func(t require.TestingT, err error, i ...any) {
		require.ErrorIs(t, err, trace.AlreadyExists("no change"))
	}
	notModifying := func(t require.TestingT, err error, i ...any) {
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
						Rules: defaultAllowRules(modules.BuildOSS)[teleport.PresetEditorRoleName],
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
						Rules: defaultAllowRules(modules.BuildOSS)[teleport.PresetEditorRoleName],
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
						MCP: &types.MCPPermissions{
							Tools: defaultMCPTools()[teleport.PresetAccessRoleName],
						},
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
						Rules: defaultAllowRules(modules.BuildOSS)[teleport.PresetAccessRoleName],
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
						Rules:                 defaultAllowRules(modules.BuildOSS)[teleport.PresetAccessRoleName],
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
						MCP: &types.MCPPermissions{
							Tools: defaultMCPTools()[teleport.PresetAccessRoleName],
						},
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
						Rules:                 defaultAllowRules(modules.BuildOSS)[teleport.PresetAccessRoleName],
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
						MCP: &types.MCPPermissions{
							Tools: defaultMCPTools()[teleport.PresetAccessRoleName],
						},
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
						Rules:                 defaultAllowRules(modules.BuildOSS)[teleport.PresetAccessRoleName],
						GitHubPermissions: []types.GitHubPermission{{
							Organizations: defaultGitHubOrgs()[teleport.PresetAccessRoleName],
						}},
						MCP: &types.MCPPermissions{
							Tools: defaultMCPTools()[teleport.PresetAccessRoleName],
						},
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
						Rules: defaultAllowRules(modules.BuildOSS)[teleport.PresetAuditorRoleName],
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
						Rules: defaultAllowRules(modules.BuildOSS)[teleport.PresetAuditorRoleName],
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
						// Missing all the node/app/database/windows labels here, they should be added
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
									types.KindLock,
									types.KindLoginRule,
									types.KindNode,
									types.KindOIDC,
									types.KindOktaImportRule,
									types.KindRole,
									types.KindSAML,
									types.KindSAMLIdPServiceProvider,
									types.KindSessionRecordingConfig,
									types.KindToken,
									types.KindTrustedCluster,
									types.KindUIConfig,
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
						KubernetesLabels:     map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
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
									types.KindLock,
									types.KindLoginRule,
									types.KindNode,
									types.KindOIDC,
									types.KindOktaImportRule,
									types.KindRole,
									types.KindSAML,
									types.KindSAMLIdPServiceProvider,
									types.KindSessionRecordingConfig,
									types.KindToken,
									types.KindTrustedCluster,
									types.KindUIConfig,
									types.KindUser,
									// The resources that already got into the main rule are still present.
									types.KindBot,
									types.KindInstaller,
								},
								Verbs: RW(),
							},
							// The missing resources got added as individual rules
							types.NewRule(types.KindDiscoveryConfig, RW()),
							types.NewRule(types.KindKubernetesCluster, RW()),
							types.NewRule(types.KindAccessMonitoringRule, RW()),
							types.NewRule(types.KindDynamicWindowsDesktop, RW()),
							types.NewRule(types.KindStaticHostUser, RW()),
							types.NewRule(types.KindWorkloadIdentity, RW()),
							types.NewRule(types.KindGitServer, RW()),
							types.NewRule(types.KindAutoUpdateConfig, RW()),
							types.NewRule(types.KindAutoUpdateVersion, RW()),
							types.NewRule(types.KindHealthCheckConfig, RW()),
							types.NewRule(types.KindVnetConfig, RW()),
							types.NewRule(types.KindIntegration, RW()),
							types.NewRule(types.KindAppAuthConfig, RW()),
							types.NewRule(types.KindInferenceModel, RW()),
							types.NewRule(types.KindInferenceSecret, RW()),
							types.NewRule(types.KindInferencePolicy, RW()),
							types.NewRule(types.KindClassifier, RW()),
							types.NewRule(types.KindRetrievalModel, RW()),
							types.NewRule(types.KindScopedToken, RW()),
							types.NewRule(access.KindScopedRole, RW()),
							types.NewRule(access.KindScopedRoleAssignment, RW()),
							types.NewRule(types.KindDatabaseObjectImportRule, RW()),
							types.NewRule(types.KindBeamsConfig, RW()),
						},
					},
				},
			},
		},
		{
			name: "access-plugin (default roles match preset rules)",
			role: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetAccessPluginRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default access plugin role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetAccessPluginRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default access plugin role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							// The missing resources got added as individual rules
							types.NewRule(types.KindAccessRequest, RO()),
							types.NewRule(types.KindAccessPluginData, RW()),
							types.NewRule(types.KindAccessMonitoringRule, RO()),
							types.NewRule(types.KindAccessList, RO()),
							types.NewRule(types.KindRole, RO()),
							types.NewRule(types.KindUser, RO()),
							types.NewRule(types.KindUserLoginState, RO()),
						},
					},
				},
			},
		},
		{
			name: "access-plugin-with-review (default roles match preset rules)",
			role: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetAccessPluginWithReviewRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default access plugin with review role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						ReviewRequests: &types.AccessReviewConditions{
							PreviewAsRoles: []string{
								teleport.PresetListAccessRequestResourcesRoleName,
							},
							SubmitForUsers: []string{"*"},
						},
					},
				},
			},
			expectedErr:    require.NoError,
			reviewNotEmpty: true,
			expected:       NewPresetAccessPluginWithReviewRole(),
		},
		{
			name: "list-access-request-resources (default roles match preset rules)",
			role: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetListAccessRequestResourcesRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default list access request resources role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetListAccessRequestResourcesRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Default list access request resources role",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							types.NewRule(types.KindNode, RO()),
							types.NewRule(types.KindApp, RO()),
							types.NewRule(types.KindDatabase, RO()),
							types.NewRule(types.KindKubernetesCluster, RO()),
						},
						AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
						DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
						GroupLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
						KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
						NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
				},
			},
		},
		{
			name:       "device-admin (missing mobile_device.create_enroll_token)",
			enterprise: true,
			role: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetDeviceAdminRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Administer trusted devices",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							types.NewRule(types.KindDevice, append(RW(), types.VerbCreateEnrollToken, types.VerbEnroll)),
						},
					},
				},
			},
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V8,
				Metadata: types.Metadata{
					Name:        teleport.PresetDeviceAdminRoleName,
					Namespace:   apidefaults.Namespace,
					Description: "Administer trusted devices",
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: []types.Rule{
							types.NewRule(types.KindDevice, append(RW(), types.VerbCreateEnrollToken, types.VerbEnroll)),
							types.NewRule(types.KindMobileDevice, []string{types.VerbCreateEnrollToken}),
						},
					},
				},
			},
		},
		{
			name: "beam-admin (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetBeamAdminRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			enterprise:  true,
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetBeamAdminRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules(modules.BuildEnterprise)[teleport.PresetBeamAdminRoleName],
					},
				},
			},
		},
		{
			name: "beam-user (enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetBeamUserRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			enterprise:  true,
			expectedErr: require.NoError,
			expected: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetBeamUserRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Rules: defaultAllowRules(modules.BuildEnterprise)[teleport.PresetBeamUserRoleName],
					},
				},
			},
		},
		{
			name: "beam-admin (not enterprise)",
			role: &types.RoleV6{
				Metadata: types.Metadata{
					Name: teleport.PresetBeamAdminRoleName,
					Labels: map[string]string{
						types.TeleportInternalResourceType: types.PresetResource,
					},
				},
			},
			expectedErr: noChange,
			expected:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buildType := modules.BuildOSS
			if test.enterprise {
				buildType = modules.BuildEnterprise
			}

			role, err := AddRoleDefaults(t.Context(), buildType, test.role)
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

// TestBeamPresetRolesAllowBeamSessionEndRead verifies the effective RBAC for
// the beam-user and beam-admin preset roles on session recording events:
//
//   - Only [events.BeamSessionEndEvent] is readable; other session-end event
//     types (SSH, Windows, Database) are denied.
//   - beam-user can only read beam sessions they own (owner == their username).
//   - beam-admin can read any beam session regardless of owner.
//
// The test exercises the same two RBAC paths that
// [ServerWithRoles.SearchSessionEvents] uses: the where-clause pushed to the
// backend as a query filter (via ExtractConditionForIdentifier), and the
// per-event CanView check (via CheckAccess on the resource rebuilt from the
// audit event).
func TestBeamPresetRolesAllowBeamSessionEndRead(t *testing.T) {
	require.Nil(t, NewPresetBeamUserRole(modules.BuildOSS), "beam-user must be enterprise-only")
	require.Nil(t, NewPresetBeamAdminRole(modules.BuildOSS), "beam-admin must be enterprise-only")

	const beamOwner = "alice"
	const otherOwner = "bob"
	ownerUser, err := types.NewUser(beamOwner)
	require.NoError(t, err)

	beamOwned := &apievents.BeamSessionEnd{
		Metadata: apievents.Metadata{
			Type: "beam.session.end",
		},
		UserMetadata: apievents.UserMetadata{User: beamOwner},
		BeamId:       "beam-owned",
	}
	beamOther := &apievents.BeamSessionEnd{
		Metadata: apievents.Metadata{
			Type: "beam.session.end",
		},
		UserMetadata: apievents.UserMetadata{User: otherOwner},
		BeamId:       "beam-other",
	}
	sshEnd := &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Type: "session.end",
		},
		ConnectionMetadata: apievents.ConnectionMetadata{Protocol: apievents.EventProtocolSSH},
		ServerMetadata:     apievents.ServerMetadata{ServerID: "node-1"},
	}
	dbEnd := &apievents.DatabaseSessionEnd{
		Metadata:         apievents.Metadata{Type: "db.session.end"},
		DatabaseMetadata: apievents.DatabaseMetadata{DatabaseService: "db-1"},
	}
	winEnd := &apievents.WindowsDesktopSessionEnd{
		Metadata:    apievents.Metadata{Type: "windows.desktop.session.end"},
		DesktopName: "desktop-1",
	}

	tests := []struct {
		name         string
		role         types.Role
		wantWhere    string
		canView      []apievents.AuditEvent
		cannotView   []apievents.AuditEvent
		filterEvents map[apievents.AuditEvent]bool // expected result of the where-clause filter
	}{
		{
			name:       "beam-user",
			role:       NewPresetBeamUserRole(modules.BuildEnterprise),
			wantWhere:  `equals(session.event, "beam.session.end") && equals(session.user, user.metadata.name)`,
			canView:    []apievents.AuditEvent{beamOwned},
			cannotView: []apievents.AuditEvent{beamOther, sshEnd, dbEnd, winEnd},
			filterEvents: map[apievents.AuditEvent]bool{
				beamOwned: true,
				beamOther: false,
				sshEnd:    false,
				dbEnd:     false,
				winEnd:    false,
			},
		},
		{
			name:       "beam-admin",
			role:       NewPresetBeamAdminRole(modules.BuildEnterprise),
			wantWhere:  `equals(session.event, "beam.session.end")`,
			canView:    []apievents.AuditEvent{beamOwned, beamOther},
			cannotView: []apievents.AuditEvent{sshEnd, dbEnd, winEnd},
			filterEvents: map[apievents.AuditEvent]bool{
				beamOwned: true,
				beamOther: true,
				sshEnd:    false,
				dbEnd:     false,
				winEnd:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.role)
			// Namespaces and other defaults are populated at storage time; do
			// the same here so RBAC evaluation matches production behavior.
			require.NoError(t, CheckAndSetDefaults(tt.role))
			// Substitute {{user.metadata.name}} in the role's templated fields
			// (BeamLabels, NodeLabels, etc.) — this mirrors what FetchRolesForUser
			// does at login.
			resolvedRole, err := ApplyTraitsWithContext(tt.role, RoleTemplateContext{Username: beamOwner})
			require.NoError(t, err)

			// Sanity-check the raw rule shape.
			var sessionRule *types.Rule
			for i, r := range resolvedRole.GetRules(types.Allow) {
				if slices.Contains(r.Resources, types.KindSession) {
					sessionRule = &resolvedRole.GetRules(types.Allow)[i]
					break
				}
			}
			require.NotNil(t, sessionRule, "role must have a KindSession rule")
			require.ElementsMatch(t, []string{types.VerbList, types.VerbRead}, sessionRule.Verbs)
			require.Equal(t, tt.wantWhere, sessionRule.Where)

			// Build an access checker that carries only this role.
			checker := NewAccessCheckerWithRoleSet(
				&AccessInfo{Username: beamOwner, Roles: []string{resolvedRole.GetName()}},
				"cluster",
				NewRoleSet(resolvedRole),
			)

			// 1. Backend-filter path: the where clause is compiled into a
			//    ToFieldsConditionConfig that the audit backend evaluates
			//    against each stored event's JSON fields.
			cond, err := checker.ExtractConditionForIdentifier(
				&Context{User: ownerUser},
				apidefaults.Namespace,
				types.KindSession,
				types.VerbList,
				SessionIdentifier,
			)
			require.NoError(t, err)
			require.NotNil(t, cond)

			condFn, err := utils.ToFieldsCondition(utils.ToFieldsConditionConfig{Expr: cond})
			require.NoError(t, err)

			for event, wantAllowed := range tt.filterEvents {
				// Emulate how audit backends store events: JSON-encode, then
				// evaluate the condition against the decoded field map. Kept
				// inline instead of using events.ToEventFields to avoid a
				// lib/services → lib/events import cycle.
				data, err := json.Marshal(event)
				require.NoError(t, err)
				var fields utils.Fields
				require.NoError(t, json.Unmarshal(data, &fields))
				got := condFn(fields)
				require.Equalf(t, wantAllowed, got,
					"backend filter for event %q: want %v, got %v",
					event.GetType(), wantAllowed, got)
			}

			// 2. Per-event CanView path: for each returned event, the auth
			//    server rebuilds the resource and re-checks access. beam-user
			//    is further scoped by BeamLabels; beam-admin allows any owner;
			//    both deny non-beam session types because neither role grants
			//    label matchers for KindNode, KindDatabase, or
			//    KindWindowsDesktop that would match the reconstructed
			//    resources.
			canView := func(ev apievents.AuditEvent) bool {
				sctx := &Context{User: ownerUser}
				sctx.ExtendWithSessionEnd(ev, checker)
				return CanViewResourceFunc(sctx)()()
			}
			for _, ev := range tt.canView {
				require.Truef(t, canView(ev),
					"expected access to event type %q", ev.GetType())
			}
			for _, ev := range tt.cannotView {
				require.Falsef(t, canView(ev),
					"did not expect access to event type %q", ev.GetType())
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
