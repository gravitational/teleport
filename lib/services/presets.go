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
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/modules"
)

// NewSystemAutomaticAccessApproverRole creates a new Role that is allowed to
// approve any Access Request. This is restricted to Teleport Enterprise, and
// returns nil in non-Enterproise builds.
func NewSystemAutomaticAccessApproverRole() types.Role {
	enterprise := modules.GetModules().BuildType() == modules.BuildEnterprise
	if !enterprise {
		return nil
	}
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.SystemAutomaticAccessApprovalRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Approves any access request",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.SystemResource,
				types.TeleportResourceRevision:     "1",
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{"*"},
				},
			},
		},
	}
	role.CheckAndSetDefaults()
	return role
}

// NewSystemAutomaticAccessBotUser returns a new User that has (via the
// the `PresetAutomaticAccessApprovalRoleName` role) the right to automatically
// approve any access requests.
//
// This user must not:
//   - Be allowed to log into the cluster
//   - Show up in user lists in WebUI
//
// TODO(tcsc): Implement/enforce above restrictions on this user
func NewSystemAutomaticAccessBotUser() types.User {
	enterprise := modules.GetModules().BuildType() == modules.BuildEnterprise
	if !enterprise {
		return nil
	}

	user := &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:        teleport.SystemAccessApproverUserName,
			Namespace:   apidefaults.Namespace,
			Description: "Used internally by Teleport to automatically approve access requests",
			Labels: map[string]string{
				types.TeleportInternalResourceType: string(types.SystemResource),
				types.TeleportResourceRevision:     "1",
			},
		},
		Spec: types.UserSpecV2{
			Roles: []string{teleport.SystemAutomaticAccessApprovalRoleName},
		},
	}
	user.CheckAndSetDefaults()
	return user
}

// NewPresetEditorRole returns a new pre-defined role for cluster
// editors who can edit cluster configuration resources.
func NewPresetEditorRole() types.Role {
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.PresetEditorRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Edit cluster configuration",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CertificateFormat: constants.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               apidefaults.EnhancedEvents(),
				RecordSession: &types.RecordSession{
					Desktop: types.NewBoolOption(false),
				},
			},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				Rules: []types.Rule{
					types.NewRule(types.KindUser, RW()),
					types.NewRule(types.KindRole, RW()),
					types.NewRule(types.KindBot, RW()),
					types.NewRule(types.KindCrownJewel, RW()),
					types.NewRule(types.KindDatabaseObjectImportRule, RW()),
					types.NewRule(types.KindOIDC, RW()),
					types.NewRule(types.KindSAML, RW()),
					types.NewRule(types.KindGithub, RW()),
					types.NewRule(types.KindOIDCRequest, RW()),
					types.NewRule(types.KindSAMLRequest, RW()),
					types.NewRule(types.KindGithubRequest, RW()),
					types.NewRule(types.KindClusterAuditConfig, RW()),
					types.NewRule(types.KindClusterAuthPreference, RW()),
					types.NewRule(types.KindAuthConnector, RW()),
					types.NewRule(types.KindClusterName, RW()),
					types.NewRule(types.KindClusterNetworkingConfig, RW()),
					types.NewRule(types.KindSessionRecordingConfig, RW()),
					types.NewRule(types.KindExternalAuditStorage, RW()),
					types.NewRule(types.KindUIConfig, RW()),
					types.NewRule(types.KindTrustedCluster, RW()),
					types.NewRule(types.KindRemoteCluster, RW()),
					types.NewRule(types.KindToken, RW()),
					types.NewRule(types.KindConnectionDiagnostic, RW()),
					types.NewRule(types.KindDatabase, RW()),
					types.NewRule(types.KindDatabaseCertificate, RW()),
					types.NewRule(types.KindInstaller, RW()),
					types.NewRule(types.KindDevice, append(RW(), types.VerbCreateEnrollToken, types.VerbEnroll)),
					types.NewRule(types.KindDatabaseService, RO()),
					types.NewRule(types.KindInstance, RO()),
					types.NewRule(types.KindLoginRule, RW()),
					types.NewRule(types.KindSAMLIdPServiceProvider, RW()),
					types.NewRule(types.KindUserGroup, RW()),
					types.NewRule(types.KindPlugin, RW()),
					types.NewRule(types.KindOktaImportRule, RW()),
					types.NewRule(types.KindOktaAssignment, RW()),
					types.NewRule(types.KindLock, RW()),
					types.NewRule(types.KindIntegration, append(RW(), types.VerbUse)),
					types.NewRule(types.KindBilling, RW()),
					types.NewRule(types.KindClusterAlert, RW()),
					types.NewRule(types.KindAccessList, RW()),
					types.NewRule(types.KindNode, RW()),
					types.NewRule(types.KindDiscoveryConfig, RW()),
					types.NewRule(types.KindSecurityReport, append(RW(), types.VerbUse)),
					types.NewRule(types.KindAuditQuery, append(RW(), types.VerbUse)),
					types.NewRule(types.KindAccessGraph, RW()),
					types.NewRule(types.KindServerInfo, RW()),
					types.NewRule(types.KindAccessMonitoringRule, RW()),
					types.NewRule(types.KindAppServer, RW()),
					types.NewRule(types.KindVnetConfig, RW()),
					types.NewRule(types.KindBotInstance, RW()),
					types.NewRule(types.KindAccessGraphSettings, RW()),
					types.NewRule(types.KindSPIFFEFederation, RW()),
					types.NewRule(types.KindNotification, RW()),
					types.NewRule(types.KindStaticHostUser, RW()),
					types.NewRule(types.KindUserTask, RW()),
					types.NewRule(types.KindContact, RW()),
					types.NewRule(types.KindWorkloadIdentity, RW()),
					types.NewRule(types.KindAutoUpdateVersion, RW()),
					types.NewRule(types.KindAutoUpdateConfig, RW()),
					types.NewRule(types.KindWorkloadIdentityX509Revocation, RW()),
				},
			},
		},
	}
	return role
}

// NewPresetAccessRole creates a role for users who are allowed to initiate
// interactive sessions.
func NewPresetAccessRole() types.Role {
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.PresetAccessRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Access cluster resources",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CertificateFormat: constants.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               apidefaults.EnhancedEvents(),
				RecordSession:     &types.RecordSession{Desktop: types.NewBoolOption(true)},
			},
			Allow: types.RoleConditions{
				Namespaces:            []string{apidefaults.Namespace},
				NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
				WindowsDesktopLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseNames:         []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:         []string{teleport.TraitInternalDBUsersVariable},
				DatabaseRoles:         []string{teleport.TraitInternalDBRolesVariable},
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.Wildcard,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
				Rules: []types.Rule{
					types.NewRule(types.KindEvent, RO()),
					{
						Resources: []string{types.KindSession},
						Verbs:     []string{types.VerbRead, types.VerbList},
						Where:     "contains(session.participants, user.metadata.name)",
					},
					types.NewRule(types.KindInstance, RO()),
					types.NewRule(types.KindClusterMaintenanceConfig, RO()),
				},
			},
		},
	}
	role.SetLogins(types.Allow, []string{teleport.TraitInternalLoginsVariable})
	role.SetWindowsLogins(types.Allow, []string{teleport.TraitInternalWindowsLoginsVariable})
	role.SetKubeUsers(types.Allow, []string{teleport.TraitInternalKubeUsersVariable})
	role.SetKubeGroups(types.Allow, []string{teleport.TraitInternalKubeGroupsVariable})
	role.SetAWSRoleARNs(types.Allow, []string{teleport.TraitInternalAWSRoleARNs})
	role.SetAzureIdentities(types.Allow, []string{teleport.TraitInternalAzureIdentities})
	role.SetGCPServiceAccounts(types.Allow, []string{teleport.TraitInternalGCPServiceAccounts})
	return role
}

// NewPresetAuditorRole returns a new pre-defined role for cluster
// auditor - someone who can review cluster events and replay sessions,
// but can't initiate interactive sessions or modify configuration.
func NewPresetAuditorRole() types.Role {
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.PresetAuditorRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Review cluster events and replay sessions",
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
				Namespaces: []string{apidefaults.Namespace},
				Rules: []types.Rule{
					types.NewRule(types.KindSession, RO()),
					types.NewRule(types.KindEvent, RO()),
					types.NewRule(types.KindSessionTracker, RO()),
					types.NewRule(types.KindClusterAlert, RO()),
					types.NewRule(types.KindInstance, RO()),
					types.NewRule(types.KindSecurityReport, append(RO(), types.VerbUse)),
					types.NewRule(types.KindAuditQuery, append(RO(), types.VerbUse)),
					types.NewRule(types.KindBotInstance, RO()),
					types.NewRule(types.KindNotification, RO()),
				},
			},
		},
	}
	role.SetLogins(types.Allow, []string{"no-login-" + uuid.New().String()})
	return role
}

// NewPresetReviewerRole returns a new pre-defined role for reviewer. The
// reviewer will be able to review all access requests.
func NewPresetReviewerRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:        teleport.PresetReviewerRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Review access requests",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: defaultAllowAccessReviewConditions(true)[teleport.PresetReviewerRoleName],
			},
		},
	}
	return role
}

// NewPresetRequesterRole returns a new pre-defined role for requester. The
// requester will be able to request all resources.
func NewPresetRequesterRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:        teleport.PresetRequesterRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Request all resources",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Request: defaultAllowAccessRequestConditions(true)[teleport.PresetRequesterRoleName],
			},
		},
	}
	return role
}

// NewPresetGroupAccessRole returns a new pre-defined role for group access -
// a role used for requesting and reviewing user group access.
func NewPresetGroupAccessRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:        teleport.PresetGroupAccessRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Have access to all user groups",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				GroupLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
				Rules: []types.Rule{
					types.NewRule(types.KindUserGroup, RO()),
				},
			},
		},
	}
	return role
}

// NewPresetDeviceAdminRole returns the preset "device-admin" role, or nil for
// non-Enterprise builds.
// The role is used to administer trusted devices.
func NewPresetDeviceAdminRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
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
	}
}

// NewPresetDeviceEnrollRole returns the preset "device-enroll" role, or nil for
// non-Enterprise builds.
// The role is used to grant device enrollment powers to users.
func NewPresetDeviceEnrollRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:        teleport.PresetDeviceEnrollRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Grant permission to enroll trusted devices",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindDevice, []string{types.VerbEnroll}),
				},
			},
		},
	}
}

// NewPresetRequireTrustedDeviceRole returns the preset "require-trusted-device"
// role, or nil for non-Enterprise builds.
// The role is used as a basis for requiring trusted device access to
// resources.
func NewPresetRequireTrustedDeviceRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V6,
		Metadata: types.Metadata{
			Name:        teleport.PresetRequireTrustedDeviceRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Require trusted device to access resources",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				DeviceTrustMode: constants.DeviceTrustModeRequired,
			},
			Allow: types.RoleConditions{
				// All SSH nodes.
				Logins: []string{"{{internal.logins}}"},
				NodeLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},

				// All k8s nodes.
				KubeGroups: []string{
					"{{internal.kubernetes_groups}}",
					// Common/example groups.
					"system:masters",
					"developers",
					"viewers",
				},
				KubernetesLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},

				// All DB nodes.
				DatabaseLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
				DatabaseNames: []string{types.Wildcard},
				DatabaseUsers: []string{types.Wildcard},
			},
		},
	}
}

// NewPresetWildcardWorkloadIdentityIssuerRole returns a new pre-defined role
// for issuing workload identities.
func NewPresetWildcardWorkloadIdentityIssuerRole() types.Role {
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.PresetWildcardWorkloadIdentityIssuerRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Issue workload identities",
			Labels: map[string]string{
				types.TeleportInternalResourceType: types.PresetResource,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				WorkloadIdentityLabels: types.Labels{
					types.Wildcard: []string{types.Wildcard},
				},
				Rules: []types.Rule{
					types.NewRule(types.KindWorkloadIdentity, RO()),
				},
			},
		},
	}
	return role
}

// SystemOktaAccessRoleName is the name of the system role that allows
// access to Okta resources. This will be used by the Okta requester role to
// search for Okta resources.
func NewSystemOktaAccessRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.SystemOktaAccessRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Request Okta resources",
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
				Rules: []types.Rule{
					types.NewRule(types.KindUserGroup, RO()),
				},
			},
		},
	}
	return role
}

// SystemOktaRequesterRoleName is a name of a system role that allows
// for requesting access to Okta resources. This differs from the requester role
// in that it allows for requesting longer lived access.
func NewSystemOktaRequesterRole() types.Role {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil
	}

	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name:        teleport.SystemOktaRequesterRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Request Okta resources",
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
	}
	return role
}

// NewPresetTerraformProviderRole returns a new pre-defined role for the Teleport Terraform provider.
// This role can edit any Terraform-supported resource.
func NewPresetTerraformProviderRole() types.Role {
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
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
				// In Teleport, you can only see what you have access to. To be able to reconcile
				// Apps, Databases, and Nodes, Terraform must be able to access them all.
				// For Databases and Nodes, Terraform cannot actually access them because it has no
				// Login/user set.
				AppLabels:      map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
				NodeLabels:     map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
				// Every resource currently supported by the Terraform provider.
				Rules: []types.Rule{
					// You must add new resources as separate rules for the
					// default rule addition logic to work properly.
					types.NewRule(types.KindAccessList, RW()),
					types.NewRule(types.KindApp, RW()),
					types.NewRule(types.KindClusterAuthPreference, RW()),
					types.NewRule(types.KindClusterMaintenanceConfig, RW()),
					types.NewRule(types.KindClusterNetworkingConfig, RW()),
					types.NewRule(types.KindDatabase, RW()),
					types.NewRule(types.KindDevice, RW()),
					types.NewRule(types.KindGithub, RW()),
					types.NewRule(types.KindLoginRule, RW()),
					types.NewRule(types.KindNode, RW()),
					types.NewRule(types.KindOIDC, RW()),
					types.NewRule(types.KindOktaImportRule, RW()),
					types.NewRule(types.KindRole, RW()),
					types.NewRule(types.KindSAML, RW()),
					types.NewRule(types.KindSessionRecordingConfig, RW()),
					types.NewRule(types.KindToken, RW()),
					types.NewRule(types.KindTrustedCluster, RW()),
					types.NewRule(types.KindUser, RW()),
					types.NewRule(types.KindBot, RW()),
					types.NewRule(types.KindInstaller, RW()),
					types.NewRule(types.KindAccessMonitoringRule, RW()),
					types.NewRule(types.KindStaticHostUser, RW()),
					types.NewRule(types.KindWorkloadIdentity, RW()),
				},
			},
		},
	}
	return role
}

// bootstrapRoleMetadataLabels are metadata labels that will be applied to each role.
// These are intended to add labels for older roles that didn't previously have them.
func bootstrapRoleMetadataLabels() map[string]map[string]string {
	return map[string]map[string]string{
		teleport.PresetAccessRoleName: {
			types.TeleportInternalResourceType: types.PresetResource,
		},
		teleport.PresetEditorRoleName: {
			types.TeleportInternalResourceType: types.PresetResource,
		},
		teleport.PresetAuditorRoleName: {
			types.TeleportInternalResourceType: types.PresetResource,
		},
		teleport.SystemOktaRequesterRoleName: {
			types.TeleportInternalResourceType: types.SystemResource,
			types.OriginLabel:                  types.OriginOkta,
		},
		// Group access, reviewer and requester are intentionally not added here as there may be
		// existing customer defined roles that have these labels.
	}
}

var defaultAllowRulesMap = map[string][]types.Rule{
	teleport.PresetAuditorRoleName:           NewPresetAuditorRole().GetRules(types.Allow),
	teleport.PresetEditorRoleName:            NewPresetEditorRole().GetRules(types.Allow),
	teleport.PresetAccessRoleName:            NewPresetAccessRole().GetRules(types.Allow),
	teleport.PresetTerraformProviderRoleName: NewPresetTerraformProviderRole().GetRules(types.Allow),
}

// defaultAllowRules has the Allow rules that should be set as default when
// they were not explicitly defined. This is used to update the current cluster
// roles when deploying a new resource. It will also update all existing roles
// on auth server restart. Rules defined in preset template should be
// exactly the same rule when added here.
func defaultAllowRules() map[string][]types.Rule {
	return defaultAllowRulesMap
}

// defaultAllowLabels has the Allow labels that should be set as default when they were not explicitly defined.
// This is used to update existing builtin preset roles with new permissions during cluster upgrades.
// The following Labels are supported:
// - AppLabels
// - DatabaseServiceLabels (db_service_labels)
// - GroupLabels
func defaultAllowLabels(enterprise bool) map[string]types.RoleConditions {
	wildcardLabels := types.Labels{types.Wildcard: []string{types.Wildcard}}
	conditions := map[string]types.RoleConditions{
		teleport.PresetAccessRoleName: {
			DatabaseServiceLabels: wildcardLabels,
			DatabaseRoles:         []string{teleport.TraitInternalDBRolesVariable},
		},
		teleport.PresetTerraformProviderRoleName: {
			AppLabels:      wildcardLabels,
			DatabaseLabels: wildcardLabels,
			NodeLabels:     wildcardLabels,
		},
	}

	if enterprise {
		conditions[teleport.SystemOktaAccessRoleName] = types.RoleConditions{
			AppLabels:   types.Labels{types.OriginLabel: []string{types.OriginOkta}},
			GroupLabels: types.Labels{types.OriginLabel: []string{types.OriginOkta}},
		}
	}

	return conditions
}

// defaultAllowAccessRequestConditions has the access request conditions that should be set as default when they were
// not explicitly defined.
func defaultAllowAccessRequestConditions(enterprise bool) map[string]*types.AccessRequestConditions {
	if enterprise {
		return map[string]*types.AccessRequestConditions{
			teleport.PresetRequesterRoleName: {
				SearchAsRoles: []string{
					teleport.PresetAccessRoleName,
					teleport.PresetGroupAccessRoleName,
				},
			},
			teleport.SystemOktaRequesterRoleName: {
				SearchAsRoles: []string{
					teleport.SystemOktaAccessRoleName,
				},
				MaxDuration: types.NewDuration(MaxAccessDuration),
			},
		}
	}

	return map[string]*types.AccessRequestConditions{}
}

// defaultAllowAccessReviewConditions has the access review conditions that should be set as default when they were
// not explicitly defined.
func defaultAllowAccessReviewConditions(enterprise bool) map[string]*types.AccessReviewConditions {
	if enterprise {
		return map[string]*types.AccessReviewConditions{
			teleport.PresetReviewerRoleName: {
				PreviewAsRoles: []string{
					teleport.PresetAccessRoleName,
					teleport.PresetGroupAccessRoleName,
				},
				Roles: []string{
					teleport.PresetAccessRoleName,
					teleport.PresetGroupAccessRoleName,
				},
			},
		}
	}

	return map[string]*types.AccessReviewConditions{}
}

// AddRoleDefaults adds default role attributes to a preset role.
// Only attributes whose resources are not already defined (either allowing or denying) are added.
func AddRoleDefaults(role types.Role) (types.Role, error) {
	changed := false

	oldLabels := role.GetAllLabels()

	// Role labels
	defaultRoleLabels, ok := bootstrapRoleMetadataLabels()[role.GetName()]
	if ok {
		metadata := role.GetMetadata()

		if metadata.Labels == nil {
			metadata.Labels = make(map[string]string, len(defaultRoleLabels))
		}
		for label, value := range defaultRoleLabels {
			if _, ok := metadata.Labels[label]; !ok {
				metadata.Labels[label] = value
				changed = true
			}
		}

		if changed {
			role.SetMetadata(metadata)
		}
	}

	labels := role.GetMetadata().Labels
	// We're specifically checking the old labels version of the Okta requester role here
	// because we're bootstrapping new labels onto the role above. By checking the old labels,
	// we can be assured that we're looking at the role as it existed before bootstrapping. If
	// the role was user-created, then this won't have the internal-resource type attached,
	// and we'll skip the rest of adding in default values.
	if role.GetName() == teleport.SystemOktaRequesterRoleName {
		labels = oldLabels
	}

	// Check if the role has a TeleportInternalResourceType attached. We do this after setting the role metadata
	// labels because we set the role metadata labels for roles that have been well established (access,
	// editor, auditor) that may not already have this label set, but we don't set it for newer roles
	// (group-access, reviewer, requester) that may have customer definitions.
	resourceType := labels[types.TeleportInternalResourceType]
	if resourceType != types.PresetResource && resourceType != types.SystemResource {
		return nil, trace.AlreadyExists("not modifying user created role")
	}

	// Resource Rules
	defaultRules, ok := defaultAllowRules()[role.GetName()]
	if ok {
		existingRules := append(role.GetRules(types.Allow), role.GetRules(types.Deny)...)

		for _, defaultRule := range defaultRules {
			if resourceBelongsToRules(existingRules, defaultRule.Resources) {
				continue
			}

			log.Debugf("Adding default allow rule %v for role %q", defaultRule, role.GetName())
			rules := role.GetRules(types.Allow)
			rules = append(rules, defaultRule)
			role.SetRules(types.Allow, rules)
			changed = true
		}
	}

	enterprise := modules.GetModules().BuildType() == modules.BuildEnterprise

	// Labels
	defaultLabels, ok := defaultAllowLabels(enterprise)[role.GetName()]
	if ok {
		for _, kind := range []string{
			types.KindApp,
			types.KindDatabase,
			types.KindDatabaseService,
			types.KindNode,
			types.KindUserGroup,
		} {
			var labels types.Labels
			switch kind {
			case types.KindApp:
				labels = defaultLabels.AppLabels
			case types.KindDatabase:
				labels = defaultLabels.DatabaseLabels
			case types.KindDatabaseService:
				labels = defaultLabels.DatabaseServiceLabels
			case types.KindNode:
				labels = defaultLabels.NodeLabels
			case types.KindUserGroup:
				labels = defaultLabels.GroupLabels
			}
			labelsUpdated, err := updateAllowLabels(role, kind, labels)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			changed = changed || labelsUpdated
		}
		if len(defaultLabels.DatabaseRoles) > 0 && len(role.GetDatabaseRoles(types.Allow)) == 0 {
			role.SetDatabaseRoles(types.Allow, defaultLabels.DatabaseRoles)
			changed = true
		}
	}

	if role.GetAccessRequestConditions(types.Allow).IsEmpty() {
		arc := defaultAllowAccessRequestConditions(enterprise)[role.GetName()]
		if arc != nil {
			role.SetAccessRequestConditions(types.Allow, *arc)
			changed = true
		}
	}

	if role.GetAccessReviewConditions(types.Allow).IsEmpty() {
		arc := defaultAllowAccessReviewConditions(enterprise)[role.GetName()]
		if arc != nil {
			role.SetAccessReviewConditions(types.Allow, *arc)
			changed = true
		}
	}

	if !changed {
		return nil, trace.AlreadyExists("no change")
	}

	return role, nil
}

func labelMatchersUnset(role types.Role, kind string) (bool, error) {
	for _, cond := range []types.RoleConditionType{types.Allow, types.Deny} {
		labelMatchers, err := role.GetLabelMatchers(cond, kind)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !labelMatchers.Empty() {
			return false, nil
		}
	}
	return true, nil
}

func resourceBelongsToRules(rules []types.Rule, resources []string) bool {
	for _, rule := range rules {
		for _, ruleResource := range rule.Resources {
			if slices.Contains(resources, ruleResource) {
				return true
			}
		}
	}

	return false
}

func updateAllowLabels(role types.Role, kind string, defaultLabels types.Labels) (bool, error) {
	var changed bool
	if unset, err := labelMatchersUnset(role, kind); err != nil {
		return false, trace.Wrap(err)
	} else if unset && len(defaultLabels) > 0 {
		role.SetLabelMatchers(types.Allow, kind, types.LabelMatchers{
			Labels: defaultLabels,
		})
		changed = true
	}

	return changed, nil
}
