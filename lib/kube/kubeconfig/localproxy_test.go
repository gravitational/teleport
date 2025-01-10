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

package kubeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestLocalProxy(t *testing.T) {
	const (
		rootKubeClusterAddr = "https://root-cluster.example.com"
		rootClusterName     = "root-cluster"
		leafClusterName     = "leaf-cluster"
	)

	kubeconfigPath, initialConfig := setup(t)
	creds, _, err := genUserKeyRing("localhost")
	require.NoError(t, err)
	exec := &ExecValues{
		TshBinaryPath: "/path/to/tsh",
	}

	// Simulate `tsh kube login`.
	require.NoError(t, Update(kubeconfigPath, Values{
		TeleportClusterName: rootClusterName,
		ClusterAddr:         rootKubeClusterAddr,
		KubeClusters:        []string{"kube1"},
		Credentials:         creds,
		Exec:                exec,
	}, false))
	require.NoError(t, Update(kubeconfigPath, Values{
		TeleportClusterName: rootClusterName,
		ClusterAddr:         rootKubeClusterAddr,
		KubeClusters:        []string{"kube2"},
		Credentials:         creds,
		Exec:                exec,
		SelectCluster:       "kube2",
	}, false))
	require.NoError(t, Update(kubeconfigPath, Values{
		TeleportClusterName: leafClusterName,
		ClusterAddr:         rootKubeClusterAddr,
		KubeClusters:        []string{"kube3"},
		Credentials:         creds,
		Namespace:           "namespace",
		Impersonate:         "as",
		ImpersonateGroups:   []string{"group1", "group2"},
		Exec:                exec,
	}, false))

	configAfterLogins, err := Load(kubeconfigPath)
	require.NoError(t, err)

	t.Run("LocalProxyClustersFromDefaultConfig", func(t *testing.T) {
		clusters := LocalProxyClustersFromDefaultConfig(configAfterLogins, rootKubeClusterAddr)
		require.ElementsMatch(t, LocalProxyClusters{
			{
				TeleportCluster: rootClusterName,
				KubeCluster:     "kube1",
			},
			{
				TeleportCluster: rootClusterName,
				KubeCluster:     "kube2",
			},
			{
				TeleportCluster:   leafClusterName,
				KubeCluster:       "kube3",
				Namespace:         "namespace",
				Impersonate:       "as",
				ImpersonateGroups: []string{"group1", "group2"},
			},
		}, clusters)
	})

	t.Run("FindTeleportClusterForLocalProxy", func(t *testing.T) {
		inputConfig := configAfterLogins.DeepCopy()

		// Simulate a scenario that kube3 is already pointing to a local proxy
		// through ProxyURL.
		inputConfig.Clusters[leafClusterName].ProxyURL = "https://localhost:8443"

		tests := []struct {
			name          string
			selectContext string
			checkResult   require.BoolAssertionFunc
			wantCluster   LocalProxyCluster
		}{
			{
				name:          "not Teleport cluster",
				selectContext: "dev",
				checkResult:   require.False,
			},
			{
				name:          "context not found",
				selectContext: "not-found",
				checkResult:   require.False,
			},
			{
				name:          "find Teleport cluster by context name",
				selectContext: rootClusterName + "-kube1",
				checkResult:   require.True,
				wantCluster: LocalProxyCluster{
					TeleportCluster: rootClusterName,
					KubeCluster:     "kube1",
				},
			},
			{
				name:          "find Teleport cluster by current context",
				selectContext: "",
				checkResult:   require.True,
				wantCluster: LocalProxyCluster{
					TeleportCluster: rootClusterName,
					KubeCluster:     "kube2",
				},
			},
			{
				name:          "skip local proxy config",
				selectContext: leafClusterName + "-kube3",
				checkResult:   require.False,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				cluster, found := FindTeleportClusterForLocalProxy(inputConfig, rootKubeClusterAddr, test.selectContext)
				test.checkResult(t, found)
				require.Equal(t, test.wantCluster, cluster)
			})
		}
	})

	t.Run("CreateLocalProxyConfig", func(t *testing.T) {
		caData := []byte("CAData")
		clientKeyData := []byte("clientKeyData")
		values := &LocalProxyValues{
			LocalProxyCAs:           map[string][]byte{rootClusterName: caData},
			TeleportKubeClusterAddr: rootKubeClusterAddr,
			LocalProxyURL:           "http://localhost:12345",
			ClientKeyData:           clientKeyData,
			Clusters: LocalProxyClusters{{
				TeleportCluster:   rootClusterName,
				KubeCluster:       "kube1",
				Namespace:         "namespace",
				Impersonate:       "as",
				ImpersonateGroups: []string{"group1", "group2"},
			}},
		}

		newConfig, err := CreateLocalProxyConfig(&initialConfig, values)
		require.NoError(t, err)
		err = Save(kubeconfigPath, *newConfig)
		require.NoError(t, err)

		generatedConfig, err := Load(kubeconfigPath)
		require.NoError(t, err)

		// Non-Teleport clusters should not change.
		wantConfig := initialConfig

		// Check for root-cluster-kube1.
		wantConfig.Clusters["root-cluster-kube1"] = &clientcmdapi.Cluster{
			ProxyURL:                 "http://localhost:12345",
			Server:                   rootKubeClusterAddr,
			CertificateAuthorityData: caData,
			TLSServerName:            "6b75626531.root-cluster",
			LocationOfOrigin:         kubeconfigPath,
			Extensions:               map[string]runtime.Object{},
		}
		wantConfig.Contexts["root-cluster-kube1"] = &clientcmdapi.Context{
			Namespace:        "namespace",
			Cluster:          "root-cluster-kube1",
			AuthInfo:         "root-cluster-kube1",
			LocationOfOrigin: kubeconfigPath,
			Extensions:       map[string]runtime.Object{},
		}
		wantConfig.AuthInfos["root-cluster-kube1"] = &clientcmdapi.AuthInfo{
			ClientCertificateData: caData,
			ClientKeyData:         clientKeyData,
			Impersonate:           "as",
			ImpersonateGroups:     []string{"group1", "group2"},
			LocationOfOrigin:      kubeconfigPath,
			Extensions:            map[string]runtime.Object{},
		}

		// Current context is updated.
		wantConfig.CurrentContext = "root-cluster-kube1"
		require.Equal(t, wantConfig, *generatedConfig)
	})
}
