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

package auth

import (
	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/pborman/uuid"
)

// NewPresetEditorRole returns a new pre-defined role for cluster
// editors who can edit cluster configuration resources.
func NewPresetEditorRole() Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:        teleport.PresetEditorRoleName,
			Namespace:   defaults.Namespace,
			Description: "Edit cluster configuration",
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Rules: []Rule{
					NewRule(KindUser, RW()),
					NewRule(KindRole, RW()),
					NewRule(KindOIDC, RW()),
					NewRule(KindSAML, RW()),
					NewRule(KindGithub, RW()),
					NewRule(KindClusterAuthPreference, RW()),
					NewRule(KindClusterConfig, RW()),
					NewRule(KindTrustedCluster, RW()),
					NewRule(KindRemoteCluster, RW()),
					NewRule(KindToken, RW()),
				},
			},
		},
	}
	return role
}

// NewPresetAccessRole creates a role for users who are allowed to initiate
// interactive sessions.
func NewPresetAccessRole() Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:        teleport.PresetAccessRoleName,
			Namespace:   defaults.Namespace,
			Description: "Access cluster resources",
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				DatabaseNames:    []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:    []string{teleport.TraitInternalDBUsersVariable},
				Rules: []Rule{
					NewRule(KindEvent, RO()),
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
func NewPresetAuditorRole() Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:        teleport.PresetAuditorRoleName,
			Namespace:   defaults.Namespace,
			Description: "Review cluster events and replay sessions",
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Rules: []Rule{
					NewRule(KindSession, RO()),
					NewRule(KindEvent, RO()),
				},
			},
			Deny: RoleConditions{
				Namespaces:       []string{Wildcard},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
			},
		},
	}
	role.SetLogins(Allow, []string{"no-login-" + uuid.New()})
	return role
}
