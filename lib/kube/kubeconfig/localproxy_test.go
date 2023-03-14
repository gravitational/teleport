// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	creds, _, err := genUserKey("localhost")
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
		require.ElementsMatch(t, []LocalProxyClusterValues{
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

	t.Run("SaveLocalProxyValues", func(t *testing.T) {
		values := &LocalProxyValues{
			LocalProxyCAPath: "/path/to/ca",
			LocalProxyAddr:   "https://localhost:12345",
			ClientKeyPath:    "/path/to/client/key",
			CliertCertPath:   "/path/to/client/cert",
			Clusters: []LocalProxyClusterValues{{
				TeleportCluster:   rootClusterName,
				KubeCluster:       "kube1",
				Namespace:         "namespace",
				Impersonate:       "as",
				ImpersonateGroups: []string{"group1", "group2"},
			}},
		}
		require.NoError(t, SaveLocalProxyValues(kubeconfigPath, rootKubeClusterAddr, configAfterLogins, values))

		generatedConfig, err := Load(kubeconfigPath)
		require.NoError(t, err)

		// Non-Teleport clusters should not change.
		wantConfig := initialConfig

		// Check for root-cluster-kube1.
		wantConfig.Clusters["root-cluster-kube1"] = &clientcmdapi.Cluster{
			Server:               "https://localhost:12345",
			CertificateAuthority: "/path/to/ca",
			TLSServerName:        "kube1.kube-teleport-localproxy-alpn.root-cluster",
			LocationOfOrigin:     kubeconfigPath,
			Extensions:           map[string]runtime.Object{},
		}
		wantConfig.Contexts["root-cluster-kube1"] = &clientcmdapi.Context{
			Namespace:        "namespace",
			Cluster:          "root-cluster-kube1",
			AuthInfo:         "root-cluster-kube1",
			LocationOfOrigin: kubeconfigPath,
			Extensions:       map[string]runtime.Object{},
		}
		wantConfig.AuthInfos["root-cluster-kube1"] = &clientcmdapi.AuthInfo{
			ClientCertificate: "/path/to/client/cert",
			ClientKey:         "/path/to/client/key",
			Impersonate:       "as",
			ImpersonateGroups: []string{"group1", "group2"},
			LocationOfOrigin:  kubeconfigPath,
			Extensions:        map[string]runtime.Object{},
		}

		// Current context is updated.
		wantConfig.CurrentContext = "root-cluster-kube1"
		require.Equal(t, wantConfig, *generatedConfig)
	})
}
