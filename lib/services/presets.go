/*
Copyright 2021 Gravitational, Inc.

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
	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	"github.com/google/uuid"
)

// NewPresetEditorRole returns a new pre-defined role for cluster
// editors who can edit cluster configuration resources.
func NewPresetEditorRole() types.Role {
	role := &types.RoleV5{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        teleport.PresetEditorRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Edit cluster configuration",
		},
		Spec: types.RoleSpecV5{
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
					types.NewRule(types.KindTrustedCluster, RW()),
					types.NewRule(types.KindRemoteCluster, RW()),
					types.NewRule(types.KindToken, RW()),
					types.NewRule(types.KindDatabaseCertificate, RW()),
				},
			},
		},
	}
	return role
}

// NewPresetAccessRole creates a role for users who are allowed to initiate
// interactive sessions.
func NewPresetAccessRole() types.Role {
	role := &types.RoleV5{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        teleport.PresetAccessRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Access cluster resources",
		},
		Spec: types.RoleSpecV5{
			Options: types.RoleOptions{
				CertificateFormat: constants.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               apidefaults.EnhancedEvents(),
				RecordSession:     &types.RecordSession{Desktop: types.NewBoolOption(true)},
			},
			Allow: types.RoleConditions{
				Namespaces:           []string{apidefaults.Namespace},
				NodeLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubernetesLabels:     types.Labels{types.Wildcard: []string{types.Wildcard}},
				WindowsDesktopLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseNames:        []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:        []string{teleport.TraitInternalDBUsersVariable},
				Rules: []types.Rule{
					types.NewRule(types.KindEvent, RO()),
					{
						Resources: []string{types.KindSession},
						Verbs:     []string{types.VerbRead, types.VerbList},
						Where:     "contains(session.participants, user.metadata.name)",
					},
				},
			},
		},
	}
	role.SetLogins(types.Allow, []string{teleport.TraitInternalLoginsVariable})
	role.SetWindowsLogins(types.Allow, []string{teleport.TraitInternalWindowsLoginsVariable})
	role.SetKubeUsers(types.Allow, []string{teleport.TraitInternalKubeUsersVariable})
	role.SetKubeGroups(types.Allow, []string{teleport.TraitInternalKubeGroupsVariable})
	role.SetAWSRoleARNs(types.Allow, []string{teleport.TraitInternalAWSRoleARNs})
	return role
}

// NewPresetAuditorRole returns a new pre-defined role for cluster
// auditor - someone who can review cluster events and replay sessions,
// but can't initiate interactive sessions or modify configuration.
func NewPresetAuditorRole() types.Role {
	role := &types.RoleV5{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        teleport.PresetAuditorRoleName,
			Namespace:   apidefaults.Namespace,
			Description: "Review cluster events and replay sessions",
		},
		Spec: types.RoleSpecV5{
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
				},
			},
		},
	}
	role.SetLogins(types.Allow, []string{"no-login-" + uuid.New().String()})
	return role
}
