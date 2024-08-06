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

package proxy

import (
	"context"
	"maps"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/services"
)

// TestWatcher verifies that kubernetes agent properly detects and applies
// changes to kube_cluster resources.
func TestWatcher(t *testing.T) {
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	reconcileCh := make(chan types.KubeClusters)
	// Setup kubernetes server that proxies one static kube cluster and
	// watches for kube_clusters with label group=a.
	testCtx := SetupTestContext(ctx, t, TestConfig{
		Clusters: []KubeClusterConfig{{"kube0", kubeMock.URL}},
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{
				"group": []string{"a"},
			}},
		},
		OnReconcile: func(kcs types.KubeClusters) {
			select {
			case reconcileCh <- kcs:
			case <-ctx.Done():
				return
			}
		},
	})

	require.Len(t, testCtx.KubeServer.fwd.kubeClusters(), 1)
	kube0 := testCtx.KubeServer.fwd.kubeClusters()[0]

	// Only kube0 should be registered initially.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create kube_cluster with label group=a.
	kube1, err := makeDynamicKubeCluster(t, "kube1", kubeMock.URL, map[string]string{"group": "a"})
	require.NoError(t, err)
	err = testCtx.AuthServer.CreateKubernetesCluster(ctx, kube1)
	require.NoError(t, err)

	// It should be registered.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Try to update kube0 which is registered statically.
	kube0Updated, err := makeDynamicKubeCluster(t, "kube0", kubeMock.URL, map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = testCtx.AuthServer.CreateKubernetesCluster(ctx, kube0Updated)
	require.NoError(t, err)

	// It should not be registered, old kube0 should remain.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create kube_cluster with label group=b.
	kube2, err := makeDynamicKubeCluster(t, "kube2", kubeMock.URL, map[string]string{"group": "b"})
	require.NoError(t, err)
	err = testCtx.AuthServer.CreateKubernetesCluster(ctx, kube2)
	require.NoError(t, err)

	// It shouldn't be registered.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update kube2 labels so it matches.
	kube2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = testCtx.AuthServer.UpdateKubernetesCluster(ctx, kube2)
	require.NoError(t, err)

	// Both should be registered now.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube1, kube2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update kube2 expiry so it gets re-registered.
	kube2.SetExpiry(time.Now().Add(1 * time.Hour))
	kube2.SetKubeconfig(newKubeConfig(t, "random", kubeMock.URL))
	err = testCtx.AuthServer.UpdateKubernetesCluster(ctx, kube2)
	require.NoError(t, err)

	// kube2 should get updated.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube1, kube2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
		// make sure credentials were updated as well.
		require.Equal(t, strings.TrimPrefix(kubeMock.URL, "https://"), testCtx.KubeServer.fwd.clusterDetails["kube2"].kubeCreds.getTargetAddr())
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update kube1 labels so it doesn't match.
	kube1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = testCtx.AuthServer.UpdateKubernetesCluster(ctx, kube1)
	require.NoError(t, err)

	// Only kube0 and kube2 should remain registered.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0, kube2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Remove kube2.
	err = testCtx.AuthServer.DeleteKubernetesCluster(ctx, kube2.GetName())
	require.NoError(t, err)

	// Only static kube_cluster should remain.
	select {
	case a := <-reconcileCh:
		require.Empty(t, cmp.Diff(types.KubeClusters{kube0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}
}

func makeDynamicKubeCluster(t *testing.T, name, url string, labels map[string]string) (*types.KubernetesClusterV3, error) {
	return makeKubeCluster(t, name, url, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	})
}

func makeKubeCluster(t *testing.T, name string, url string, labels map[string]string, additionalLabels map[string]string) (*types.KubernetesClusterV3, error) {
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, additionalLabels)
	return types.NewKubernetesClusterV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.KubernetesClusterSpecV3{
		Kubeconfig: newKubeConfig(t, name, url),
	})
}

func newKubeConfig(t *testing.T, name, url string) []byte {
	kubeConf := clientcmdapi.NewConfig()

	kubeConf.Clusters[name] = &clientcmdapi.Cluster{
		Server:                url,
		InsecureSkipTLSVerify: true,
	}
	kubeConf.AuthInfos[name] = &clientcmdapi.AuthInfo{}

	kubeConf.Contexts[name] = &clientcmdapi.Context{
		Cluster:  name,
		AuthInfo: name,
	}

	buf, err := clientcmd.Write(*kubeConf)
	require.NoError(t, err)
	return buf
}
