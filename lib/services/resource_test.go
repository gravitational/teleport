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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestParseShortcut will test parsing of shortcuts.
func TestParseShortcut(t *testing.T) {
	tests := map[string]struct {
		expectedOutput string
		expectedErr    bool
	}{
		"role":  {expectedOutput: types.KindRole},
		"roles": {expectedOutput: types.KindRole},

		"namespace":  {expectedOutput: types.KindNamespace},
		"namespaces": {expectedOutput: types.KindNamespace},
		"ns":         {expectedOutput: types.KindNamespace},

		"auth_server":  {expectedOutput: types.KindAuthServer},
		"auth_servers": {expectedOutput: types.KindAuthServer},
		"auth":         {expectedOutput: types.KindAuthServer},

		"proxy":   {expectedOutput: types.KindProxy},
		"proxies": {expectedOutput: types.KindProxy},

		"node":  {expectedOutput: types.KindNode},
		"nodes": {expectedOutput: types.KindNode},

		"oidc": {expectedOutput: types.KindOIDCConnector},

		"saml": {expectedOutput: types.KindSAMLConnector},

		"github": {expectedOutput: types.KindGithubConnector},

		"connector":  {expectedOutput: types.KindConnectors},
		"connectors": {expectedOutput: types.KindConnectors},

		"user":  {expectedOutput: types.KindUser},
		"users": {expectedOutput: types.KindUser},

		"cert_authority":   {expectedOutput: types.KindCertAuthority},
		"cert_authorities": {expectedOutput: types.KindCertAuthority},
		"cas":              {expectedOutput: types.KindCertAuthority},

		"tunnel":          {expectedOutput: types.KindReverseTunnel},
		"reverse_tunnels": {expectedOutput: types.KindReverseTunnel},
		"rts":             {expectedOutput: types.KindReverseTunnel},

		"trusted_cluster": {expectedOutput: types.KindTrustedCluster},
		"tc":              {expectedOutput: types.KindTrustedCluster},
		"cluster":         {expectedOutput: types.KindTrustedCluster},
		"clusters":        {expectedOutput: types.KindTrustedCluster},

		"cluster_auth_preference":            {expectedOutput: types.KindClusterAuthPreference},
		"cluster_authentication_preferences": {expectedOutput: types.KindClusterAuthPreference},
		"cap":                                {expectedOutput: types.KindClusterAuthPreference},

		"cluster_networking_config": {expectedOutput: types.KindClusterNetworkingConfig},
		"networking_config":         {expectedOutput: types.KindClusterNetworkingConfig},
		"networking":                {expectedOutput: types.KindClusterNetworkingConfig},
		"net_config":                {expectedOutput: types.KindClusterNetworkingConfig},
		"netconfig":                 {expectedOutput: types.KindClusterNetworkingConfig},

		"session_recording_config": {expectedOutput: types.KindSessionRecordingConfig},
		"recording_config":         {expectedOutput: types.KindSessionRecordingConfig},
		"session_recording":        {expectedOutput: types.KindSessionRecordingConfig},
		"rec_config":               {expectedOutput: types.KindSessionRecordingConfig},
		"recconfig":                {expectedOutput: types.KindSessionRecordingConfig},

		"remote_cluster":  {expectedOutput: types.KindRemoteCluster},
		"remote_clusters": {expectedOutput: types.KindRemoteCluster},
		"rc":              {expectedOutput: types.KindRemoteCluster},
		"rcs":             {expectedOutput: types.KindRemoteCluster},

		"semaphore":  {expectedOutput: types.KindSemaphore},
		"semaphores": {expectedOutput: types.KindSemaphore},
		"sem":        {expectedOutput: types.KindSemaphore},
		"sems":       {expectedOutput: types.KindSemaphore},

		"kube_cluster":  {expectedOutput: types.KindKubernetesCluster},
		"kube_clusters": {expectedOutput: types.KindKubernetesCluster},

		"kube_server":  {expectedOutput: types.KindKubeServer},
		"kube_servers": {expectedOutput: types.KindKubeServer},

		"lock":  {expectedOutput: types.KindLock},
		"locks": {expectedOutput: types.KindLock},

		"db_server": {expectedOutput: types.KindDatabaseServer},

		"network_restrictions": {expectedOutput: types.KindNetworkRestrictions},

		"db": {expectedOutput: types.KindDatabase},

		"app":  {expectedOutput: types.KindApp},
		"apps": {expectedOutput: types.KindApp},

		"windows_desktop_service": {expectedOutput: types.KindWindowsDesktopService},
		"windows_service":         {expectedOutput: types.KindWindowsDesktopService},
		"win_desktop_service":     {expectedOutput: types.KindWindowsDesktopService},
		"win_service":             {expectedOutput: types.KindWindowsDesktopService},

		"windows_desktop": {expectedOutput: types.KindWindowsDesktop},
		"win_desktop":     {expectedOutput: types.KindWindowsDesktop},

		"token":  {expectedOutput: types.KindToken},
		"tokens": {expectedOutput: types.KindToken},

		"installer": {expectedOutput: types.KindInstaller},

		"db_service":  {expectedOutput: types.KindDatabaseService},
		"db_services": {expectedOutput: types.KindDatabaseService},

		"login_rule":  {expectedOutput: types.KindLoginRule},
		"login_rules": {expectedOutput: types.KindLoginRule},

		"saml_idp_service_provider":  {expectedOutput: types.KindSAMLIdPServiceProvider},
		"saml_idp_service_providers": {expectedOutput: types.KindSAMLIdPServiceProvider},
		"saml_sp":                    {expectedOutput: types.KindSAMLIdPServiceProvider},
		"saml_sps":                   {expectedOutput: types.KindSAMLIdPServiceProvider},

		"user_group":  {expectedOutput: types.KindUserGroup},
		"user_groups": {expectedOutput: types.KindUserGroup},
		"usergroup":   {expectedOutput: types.KindUserGroup},
		"usergroups":  {expectedOutput: types.KindUserGroup},

		"device":  {expectedOutput: types.KindDevice},
		"devices": {expectedOutput: types.KindDevice},

		"okta_import_rule":  {expectedOutput: types.KindOktaImportRule},
		"okta_import_rules": {expectedOutput: types.KindOktaImportRule},
		"oktaimportrule":    {expectedOutput: types.KindOktaImportRule},
		"oktaimportrules":   {expectedOutput: types.KindOktaImportRule},

		"okta_assignment":  {expectedOutput: types.KindOktaAssignment},
		"okta_assignments": {expectedOutput: types.KindOktaAssignment},
		"oktaassignment":   {expectedOutput: types.KindOktaAssignment},
		"oktaassignments":  {expectedOutput: types.KindOktaAssignment},

		"access_list":  {expectedOutput: types.KindAccessList},
		"access_lists": {expectedOutput: types.KindAccessList},
		"accesslist":   {expectedOutput: types.KindAccessList},
		"accesslists":  {expectedOutput: types.KindAccessList},

		"SamL_IDP_sERVICe_proVidER": {expectedOutput: types.KindSAMLIdPServiceProvider},

		"unknown_type": {expectedErr: true},
	}

	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			output, err := ParseShortcut(input)

			if test.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedOutput, output)
			}
		})
	}
}

func Test_setResourceName(t *testing.T) {
	tests := []struct {
		name           string
		meta           types.Metadata
		overrideLabels []string
		firstNamePart  string
		extraNameParts []string
		want           types.Metadata
	}{
		{
			name:           "no override, one part name",
			meta:           types.Metadata{},
			firstNamePart:  "foo",
			extraNameParts: nil,
			want:           types.Metadata{Name: "foo"},
		},
		{
			name:           "no override, multi part name",
			meta:           types.Metadata{},
			firstNamePart:  "foo",
			extraNameParts: []string{"bar", "baz"},
			want:           types.Metadata{Name: "foo-bar-baz"},
		},
		{
			name:           "override by generic cloud label, one part name",
			meta:           types.Metadata{Labels: map[string]string{types.AWSDatabaseNameOverrideLabels[0]: "gizmo"}},
			overrideLabels: types.AWSDatabaseNameOverrideLabels,
			firstNamePart:  "foo",
			extraNameParts: nil,
			want:           types.Metadata{Name: "gizmo", Labels: map[string]string{types.AWSDatabaseNameOverrideLabels[0]: "gizmo"}},
		},
		{
			name:           "override by original AWS label, one part name",
			meta:           types.Metadata{Labels: map[string]string{types.AWSDatabaseNameOverrideLabels[1]: "gizmo"}},
			overrideLabels: types.AWSDatabaseNameOverrideLabels,
			firstNamePart:  "foo",
			extraNameParts: nil,
			want:           types.Metadata{Name: "gizmo", Labels: map[string]string{types.AWSDatabaseNameOverrideLabels[1]: "gizmo"}},
		},
		{
			name:           "override, multi part name",
			meta:           types.Metadata{Labels: map[string]string{types.AzureDatabaseNameOverrideLabel: "gizmo"}},
			overrideLabels: []string{types.AzureDatabaseNameOverrideLabel},
			firstNamePart:  "foo",
			extraNameParts: []string{"bar", "baz"},
			want:           types.Metadata{Name: "gizmo-bar-baz", Labels: map[string]string{types.AzureDatabaseNameOverrideLabel: "gizmo"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setResourceName(tt.overrideLabels, tt.meta, tt.firstNamePart, tt.extraNameParts...)
			require.Equal(t, tt.want, result)
		})
	}
}
