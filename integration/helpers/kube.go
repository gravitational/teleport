// Copyright 2022 Gravitational, Inc
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

package helpers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func EnableKubernetesService(t *testing.T, config *servicecfg.Config) {
	config.Kube.KubeconfigPath = filepath.Join(t.TempDir(), "kube_config")
	require.NoError(t, EnableKube(t, config, "teleport-cluster"))
}

func EnableKube(t *testing.T, config *servicecfg.Config, clusterName string) error {
	kubeConfigPath := config.Kube.KubeconfigPath
	if kubeConfigPath == "" {
		return trace.BadParameter("missing kubeconfig path")
	}

	genKubeConfig(t, kubeConfigPath, clusterName)
	config.Kube.Enabled = true
	config.Kube.ListenAddr = utils.MustParseAddr(NewListener(t, service.ListenerKube, &config.FileDescriptors))
	return nil
}

// genKubeConfig generates a kubeconfig file for a given cluster based on the
// kubeMock server.
func genKubeConfig(t *testing.T, kubeconfigPath, clusterName string) {
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, kubeMock.Close())
	})
	cfg := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                kubeMock.URL,
				InsecureSkipTLSVerify: true,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			clusterName: {},
		},
		Contexts: map[string]*clientcmdapi.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: clusterName,
			},
		},
	}
	err = kubeconfig.Save(kubeconfigPath, cfg)
	require.NoError(t, err)
}

// GetKubeClusters gets all kubernetes clusters accessible from a given auth server.
func GetKubeClusters(t *testing.T, as *auth.Server) []types.KubeCluster {
	ctx := context.Background()
	resources, err := apiclient.GetResourcesWithFilters(ctx, as, proto.ListResourcesRequest{
		ResourceType: types.KindKubeServer,
	})
	require.NoError(t, err)
	kss, err := types.ResourcesWithLabels(resources).AsKubeServers()
	require.NoError(t, err)

	clusters := make([]types.KubeCluster, 0)
	for _, ks := range kss {
		clusters = append(clusters, ks.GetCluster())
	}
	return clusters
}
