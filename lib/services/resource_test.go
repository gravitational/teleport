/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParseShortcut(t *testing.T) {
	checks := map[string]bool{}
	tests := []struct {
		name   string
		inputs []string
		output string
		err    bool
	}{
		{
			name:   "role",
			inputs: []string{types.KindRole, "roles"},
			output: types.KindRole,
		},
		{
			name:   "namespace",
			inputs: []string{types.KindNamespace, "namespaces", "ns"},
			output: types.KindNamespace,
		},
		{
			name:   "auth server",
			inputs: []string{types.KindAuthServer, "auth_servers", "auth"},
			output: types.KindAuthServer,
		},
		{
			name:   "proxy",
			inputs: []string{types.KindProxy, "proxies"},
			output: types.KindProxy,
		},
		{
			name:   "node",
			inputs: []string{types.KindNode, "nodes"},
			output: types.KindNode,
		},
		{
			name:   "oidc connecto",
			inputs: []string{types.KindOIDCConnector},
			output: types.KindOIDCConnector,
		},
		{
			name:   "saml connector",
			inputs: []string{types.KindSAMLConnector},
			output: types.KindSAMLConnector,
		},
		{
			name:   "github connector",
			inputs: []string{types.KindGithubConnector},
			output: types.KindGithubConnector,
		},
		{
			name:   "connectors",
			inputs: []string{types.KindConnectors, "connector"},
			output: types.KindConnectors,
		},
		{
			name:   "user",
			inputs: []string{types.KindUser, "users"},
			output: types.KindUser,
		},
		{
			name:   "cert authority",
			inputs: []string{types.KindCertAuthority, "cert_authorities", "cas"},
			output: types.KindCertAuthority,
		},
		{
			name:   "reverse tunnel",
			inputs: []string{types.KindReverseTunnel, "reverse_tunnels", "rts"},
			output: types.KindReverseTunnel,
		},
		{
			name:   "trusted cluster",
			inputs: []string{types.KindTrustedCluster, "tc", "cluster", "clusters"},
			output: types.KindTrustedCluster,
		},
		{
			name:   "cluster authentication preference",
			inputs: []string{types.KindClusterAuthPreference, "cluster_authentication_preferences", "cap"},
			output: types.KindClusterAuthPreference,
		},
		{
			name:   "cluster networking config",
			inputs: []string{types.KindClusterNetworkingConfig, "networking_config", "networking", "net_config", "netconfig"},
			output: types.KindClusterNetworkingConfig,
		},
		{
			name:   "session recording config",
			inputs: []string{types.KindSessionRecordingConfig, "recording_config", "session_recording", "rec_config", "recconfig"},
			output: types.KindSessionRecordingConfig,
		},
		{
			name:   "remote cluster",
			inputs: []string{types.KindRemoteCluster, "remote_clusters", "rc", "rcs"},
			output: types.KindRemoteCluster,
		},
		{
			name:   "semaphore",
			inputs: []string{types.KindSemaphore, "semaphores", "sem", "sems"},
			output: types.KindSemaphore,
		},
		{
			name:   "kube cluster",
			inputs: []string{types.KindKubernetesCluster, "kube_clusters"},
			output: types.KindKubernetesCluster,
		},
		{
			name:   "kube service",
			inputs: []string{types.KindKubeService, "kube_services"},
			output: types.KindKubeService,
		},
		{
			name:   "kube server",
			inputs: []string{types.KindKubeServer, "kube_servers"},
			output: types.KindKubeServer,
		},
		{
			name:   "lock",
			inputs: []string{types.KindLock, "locks"},
			output: types.KindLock,
		},
		{
			name:   "database server",
			inputs: []string{types.KindDatabaseServer},
			output: types.KindDatabaseServer,
		},
		{
			name:   "network restrictions",
			inputs: []string{types.KindNetworkRestrictions},
			output: types.KindNetworkRestrictions,
		},
		{
			name:   "database",
			inputs: []string{types.KindDatabase},
			output: types.KindDatabase,
		},
		{
			name:   "app",
			inputs: []string{types.KindApp, "apps"},
			output: types.KindApp,
		},
		{
			name:   "windows desktop service",
			inputs: []string{types.KindWindowsDesktopService, "windows_service", "win_desktop_service", "win_service"},
			output: types.KindWindowsDesktopService,
		},
		{
			name:   "windows desktop",
			inputs: []string{types.KindWindowsDesktop, "win_desktop"},
			output: types.KindWindowsDesktop,
		},
		{
			name:   "token",
			inputs: []string{types.KindToken, "tokens"},
			output: types.KindToken,
		},
		{
			name:   "installer",
			inputs: []string{types.KindInstaller},
			output: types.KindInstaller,
		},
		{
			name:   "error",
			inputs: []string{"non-existent shortcut"},
			output: "",
			err:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, input := range test.inputs {
				output, err := ParseShortcut(input)
				require.Equal(t, test.output, output)
				if test.err {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					checks[input] = true
				}
			}
		})
	}

	require.Len(t, checks, len(shortcuts), "not all shortcuts were tested")
}
