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

package local

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestKubernetesCRUD tests backend operations with kubernetes resources.
func TestKubernetesCRUD(t *testing.T) {
	ctx := t.Context()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewKubernetesService(backend)
	require.NoError(t, err)

	// Create a few couple kube clusters.
	kubeCluster1, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "c1",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeCluster2, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "c2",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeCluster3, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "c3",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	testBasicFlow(t, service, []types.KubeCluster{
		kubeCluster1,
		kubeCluster2,
		kubeCluster3,
	})
}

// TestScopedKubeClusterCRUD tests backend operations with kubernetes resources.
func TestScopedKubeClusterCRUD(t *testing.T) {
	ctx := t.Context()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewKubernetesService(backend)
	require.NoError(t, err)

	const (
		scope           = "/aa"
		orthogonalScope = "/bb"
	)

	// Create a couple kube clusters.
	unscopedCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube-cluster",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	scopedCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube-cluster",
	}, types.KubernetesClusterSpecV3{}, types.KubeClusterWithScope(scope))
	require.NoError(t, err)
	orthogonalCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube-cluster",
	}, types.KubernetesClusterSpecV3{}, types.KubeClusterWithScope(orthogonalScope))
	require.NoError(t, err)

	clusters := []types.KubeCluster{
		unscopedCluster,
		scopedCluster,
		orthogonalCluster,
	}
	testBasicFlow(t, service, clusters)

	// test new kube cluster methods
	//
	// Create all clusters
	for _, cluster := range clusters {
		err = service.CreateKubernetesCluster(ctx, cluster)
		require.NoError(t, err)
	}

	// ensure each cluster can be updated and fetched
	diffopt := cmpopts.IgnoreFields(types.Metadata{}, "Revision")
	for _, cluster := range clusters {
		cluster.SetStaticLabels(map[string]string{"updated": "updated"})
		err := service.UpdateKubernetesCluster(ctx, cluster)
		require.NoError(t, err)

		res, err := service.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Scope: cluster.GetScope(),
			Name:  cluster.GetName(),
		}.Build())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(cluster, res, diffopt))
		require.Equal(t, "updated", res.GetStaticLabels()["updated"])
	}

	// ensure all clusters can be listed
	list, nextToken, err := service.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
		PageSize: 10,
	}.Build())
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, list, 3)

	// ensure scope filtering works
	list, nextToken, err = service.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
		PageSize: 10,
		ScopeFilter: scopesv1.Filter_builder{
			Scope: scopedCluster.GetScope(),
			Mode:  scopesv1.Mode_MODE_EXACT,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, list, 1)
	require.Empty(t, cmp.Diff(scopedCluster, list[0], diffopt))

	// ensure unscoped filtering works
	list, nextToken, err = service.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
		PageSize: 10,
		ScopeFilter: scopesv1.Filter_builder{
			Mode: scopesv1.Mode_MODE_UNSCOPED,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, list, 1)
	require.Empty(t, cmp.Diff(unscopedCluster, list[0], diffopt))

	// ensure clusters can be deleted
	for _, cluster := range clusters {
		cluster.SetStaticLabels(map[string]string{"updated": "updated"})
		err := service.UpdateKubernetesCluster(ctx, cluster)
		require.NoError(t, err)

		err = service.DeleteKubeCluster(ctx, presencev1.DeleteKubeClusterRequest_builder{
			Scope: cluster.GetScope(),
			Name:  cluster.GetName(),
		}.Build())
		require.NoError(t, err)

		_, err = service.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Scope: cluster.GetScope(),
			Name:  cluster.GetName(),
		}.Build())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected not found error after kube cluster deletion")
	}
}

func testBasicFlow(t *testing.T, service *KubernetesService, clusters []types.KubeCluster) {
	t.Helper()
	if len(clusters) < 3 {
		require.FailNow(t, "need at least 3 clusters to run basic flow test")
	}

	t.Run("basic methods", func(t *testing.T) {
		ctx := t.Context()
		// We should start out with no clusters
		out, err := service.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, out)

		// Create all clusters
		for _, cluster := range clusters {
			err = service.CreateKubernetesCluster(ctx, cluster)
			require.NoError(t, err)
		}

		diffopt := cmpopts.IgnoreFields(types.Metadata{}, "Revision")

		// Fetch all clusters
		out, err = service.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusters, out, diffopt))

		// List with page limit
		page1, page2Start, err := service.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
			PageSize:  2,
			PageToken: "",
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, page2Start)
		require.Len(t, page1, 2)

		// List with start
		page2, next, err := service.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
			PageSize:  2,
			PageToken: page2Start,
		}.Build())
		require.NoError(t, err)
		require.Empty(t, next)
		require.Len(t, page2, 1)
		require.Empty(t, cmp.Diff(clusters, append(page1, page2...), diffopt))

		// Range over all
		out, err = stream.Collect(service.RangeKubeClusters(ctx, nil))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusters, out, diffopt))

		// Range with lower bound
		out, err = stream.Collect(service.RangeKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
			PageToken: page2Start,
		}.Build()))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(page2, out, diffopt))

		// Fetch a specific kube cluster
		cluster, err := service.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Scope: clusters[1].GetScope(),
			Name:  clusters[1].GetName(),
		}.Build())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusters[1], cluster, diffopt))

		// Try to fetch a kube cluster that doesn't exist
		_, err = service.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Name: "doesnotexist",
		}.Build())
		require.ErrorAs(t, err, new(*trace.NotFoundError))

		// Try to create the same kube cluster
		err = service.CreateKubernetesCluster(ctx, clusters[0])
		require.ErrorAs(t, err, new(*trace.AlreadyExistsError))

		// Update a kube cluster
		clusters[0].SetStaticLabels(map[string]string{"updated": "yes"})
		err = service.UpdateKubernetesCluster(ctx, clusters[0])
		require.NoError(t, err)
		cluster, err = service.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Scope: clusters[0].GetScope(),
			Name:  clusters[0].GetName(),
		}.Build())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusters[0], cluster, diffopt))

		// Delete a Kubernetes.
		err = service.DeleteKubeCluster(ctx, presencev1.DeleteKubeClusterRequest_builder{
			Scope: clusters[0].GetScope(),
			Name:  clusters[0].GetName(),
		}.Build())
		require.NoError(t, err)

		expectedClusters := []types.KubeCluster{clusters[1], clusters[2]}
		out, err = service.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(expectedClusters, out, diffopt))

		// Try to delete a Kubernetes that doesn't exist.
		err = service.DeleteKubeCluster(ctx, presencev1.DeleteKubeClusterRequest_builder{
			Name: "doesnotexist",
		}.Build())
		require.ErrorAs(t, err, new(*trace.NotFoundError))

		// Delete all Kubernetess.
		err = service.DeleteAllKubernetesClusters(ctx)
		require.NoError(t, err)
		out, err = service.GetKubernetesClusters(ctx)
		require.NoError(t, err)
		require.Empty(t, out)
	})
}
