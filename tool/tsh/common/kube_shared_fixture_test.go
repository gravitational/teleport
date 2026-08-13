/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	kubeserver "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// Kube cluster names registered by the fixture.
const (
	sharedRootKubeCluster1              = "root-cluster"
	sharedRootKubeCluster2              = "first-cluster"
	sharedLeafKubeCluster               = "leaf-cluster-some-suffix-added-by-discovery-service"
	sharedLeafKubeClusterDiscoveredName = "leaf-cluster"
)

type kubeFixtureKey struct {
	multiplexMode bool
}

var (
	kubeFixtureMu sync.Mutex
	kubeFixtures  = map[kubeFixtureKey]*suite{}
)

// getKubeFixture returns the root+leaf Teleport suite for key, building it at most once per distinct key.
// Bring-up costs ~4-5s and dominates TestKube/TestKubeLogin, whose bodies run in milliseconds,
// so paying it once rather than per iteration is what keeps the package under the timeout at `-count 100`.
func getKubeFixture(t *testing.T, key kubeFixtureKey) *suite {
	t.Helper()
	kubeFixtureMu.Lock()
	defer kubeFixtureMu.Unlock()
	if s, ok := kubeFixtures[key]; ok {
		return s
	}

	rootLabels := map[string]string{
		"label1": "val1",
		"ultra_long_label_for_teleport_kubernetes_service_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_service_list_kube_clusters_method",
	}
	leafLabels := map[string]string{
		"label1": "val1",
		"ultra_long_label_for_teleport_kubernetes_service_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_service_list_kube_clusters_method",
		// mock a discovered kube cluster in the leaf Teleport cluster.
		types.DiscoveredNameLabel: sharedLeafKubeClusterDiscoveredName,
	}

	s := newTestSuite(t,
		withSharedFixture(),
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			if key.multiplexMode {
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			}
			cfg.InsecureMode = true
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newSharedKubeConfigFile(t, sharedRootKubeCluster1, sharedRootKubeCluster2)
			cfg.Kube.StaticLabels = rootLabels
			cfg.Proxy.Kube.Enabled = true
			cfg.Proxy.Kube.ListenAddr = *utils.MustParseAddr(localListenerAddr())
			cfg.SSH.Enabled = false
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
				if key.multiplexMode {
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}
				cfg.InsecureMode = true
				cfg.Kube.Enabled = true
				cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
				cfg.Kube.KubeconfigPath = newSharedKubeConfigFile(t, sharedLeafKubeCluster)
				cfg.Kube.StaticLabels = leafLabels
				cfg.SSH.Enabled = false
			},
		),
		withValidationFunc(func(s *suite) bool {
			// Wait for cache propagation of the kubernetes resources before proceeding with the tests.
			var foundRoot1, foundRoot2, foundLeaf bool
			for ks := range s.root.GetAuthServer().UnifiedResourceCache.KubernetesServers(t.Context(), services.UnifiedResourcesIterateParams{}) {
				foundRoot1 = foundRoot1 || ks.GetCluster().GetName() == sharedRootKubeCluster1
				foundRoot2 = foundRoot2 || ks.GetCluster().GetName() == sharedRootKubeCluster2
			}

			for ks := range s.leaf.GetAuthServer().UnifiedResourceCache.KubernetesServers(t.Context(), services.UnifiedResourcesIterateParams{}) {
				foundLeaf = foundLeaf || ks.GetCluster().GetName() == sharedLeafKubeCluster
			}

			return foundRoot1 && foundRoot2 && foundLeaf
		}),
	)

	kubeFixtures[key] = s
	return s
}

func newSharedKubeConfigFile(t *testing.T, clusterNames ...string) string {
	return buildKubeConfigFile(t, sharedTempDir(t), newSharedKubeSelfSubjectServer, clusterNames...)
}

func newSharedKubeSelfSubjectServer(t *testing.T) string {
	srv, err := kubeserver.NewKubeAPIMock()
	require.NoError(t, err)
	registerSharedFixtureTeardown(func() { srv.Close() })
	return srv.URL
}
