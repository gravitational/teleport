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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/pborman/uuid"
)

// NewPresetEditorRole returns a new pre-defined role for cluster
// editors who can edit cluster configuration resources.
func NewPresetEditorRole() types.Role {
	role := &types.RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: types.Metadata{
			Name:        teleport.PresetEditorRoleName,
			Namespace:   defaults.Namespace,
			Description: "Edit cluster configuration",
		},
		Spec: types.RoleSpecV3{
			Options: types.RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(defaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: types.RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Rules: []types.Rule{
					types.NewRule(KindUser, RW()),
					types.NewRule(KindRole, RW()),
					types.NewRule(KindOIDC, RW()),
					types.NewRule(KindSAML, RW()),
					types.NewRule(KindGithub, RW()),
					types.NewRule(KindClusterAuthPreference, RW()),
					types.NewRule(KindClusterConfig, RW()),
					types.NewRule(KindTrustedCluster, RW()),
					types.NewRule(KindRemoteCluster, RW()),
					types.NewRule(KindToken, RW()),
				},
			},
		},
	}
	return role
}

// NewPresetAccessRole creates a role for users who are allowed to initiate
// interactive sessions.
func NewPresetAccessRole() types.Role {
	role := &types.RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: types.Metadata{
			Name:        teleport.PresetAccessRoleName,
			Namespace:   defaults.Namespace,
			Description: "Access cluster resources",
		},
		Spec: types.RoleSpecV3{
			Options: types.RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(defaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: types.RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       types.Labels{Wildcard: []string{Wildcard}},
				AppLabels:        types.Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: types.Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   types.Labels{Wildcard: []string{Wildcard}},
				DatabaseNames:    []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:    []string{teleport.TraitInternalDBUsersVariable},
				Rules: []types.Rule{
					types.NewRule(KindEvent, RO()),
				},
			},
		},
	}
	role.SetLogins(Allow, []string{teleport.TraitInternalLoginsVariable})
	role.SetKubeUsers(Allow, []string{teleport.TraitInternalKubeUsersVariable})
	role.SetKubeGroups(Allow, []string{teleport.TraitInternalKubeGroupsVariable})
	return role
}

// NewPresetAuditorRole returns a new pre-defined role for cluster
// auditor - someone who can review cluster events and replay sessions,
// but can't initiate interactive sessions or modify configuration.
func NewPresetAuditorRole() types.Role {
	role := &types.RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: types.Metadata{
			Name:        teleport.PresetAuditorRoleName,
			Namespace:   defaults.Namespace,
			Description: "Review cluster events and replay sessions",
		},
		Spec: types.RoleSpecV3{
			Options: types.RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(defaults.MaxCertDuration),
			},
			Allow: types.RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Rules: []types.Rule{
					types.NewRule(KindSession, RO()),
					types.NewRule(KindEvent, RO()),
				},
			},
			Deny: types.RoleConditions{
				Namespaces:       []string{Wildcard},
				NodeLabels:       types.Labels{Wildcard: []string{Wildcard}},
				AppLabels:        types.Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: types.Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   types.Labels{Wildcard: []string{Wildcard}},
			},
		},
	}
	role.SetLogins(Allow, []string{"no-login-" + uuid.New()})
	return role
}
