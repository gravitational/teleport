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
