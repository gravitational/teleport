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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestKubernetesCRUD tests backend operations with kubernetes resources.
func TestKubernetesCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewKubernetesService(backend)

	// Create a couple kube clusters.
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

	// Initially we expect no Kubernetess.
	out, err := service.GetKubernetesClusters(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both Kubernetess.
	err = service.CreateKubernetesCluster(ctx, kubeCluster1)
	require.NoError(t, err)
	err = service.CreateKubernetesCluster(ctx, kubeCluster2)
	require.NoError(t, err)
	err = service.CreateKubernetesCluster(ctx, kubeCluster3)
	require.NoError(t, err)

	expectedAll := []types.KubeCluster{kubeCluster1, kubeCluster2, kubeCluster3}
	diffopt := cmpopts.IgnoreFields(types.Metadata{}, "Revision")

	// Fetch all Kubernetess.
	out, err = service.GetKubernetesClusters(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedAll, out, diffopt))

	// List with page limit
	page1, page2Start, err := service.ListKubernetesClusters(ctx, 2, "")
	require.NoError(t, err)
	require.NotEmpty(t, page2Start)
	require.Len(t, page1, 2)

	// List with start
	page2, next, err := service.ListKubernetesClusters(ctx, 2, page2Start)
	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, page2, 1)
	require.Empty(t, cmp.Diff(expectedAll, append(page1, page2...), diffopt))

	// Range over all
	out, err = stream.Collect(service.RangeKubernetesClusters(ctx, "", ""))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedAll, out, diffopt))

	// Range with upper bound
	out, err = stream.Collect(service.RangeKubernetesClusters(ctx, "", page2Start))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(page1, out, diffopt))

	// Range with lower bound
	out, err = stream.Collect(service.RangeKubernetesClusters(ctx, page2Start, ""))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(page2, out, diffopt))

	// Fetch a specific Kubernetes.
	cluster, err := service.GetKubernetesCluster(ctx, kubeCluster2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(kubeCluster2, cluster, diffopt))

	// Try to fetch a Kubernetes that doesn't exist.
	_, err = service.GetKubernetesCluster(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same Kubernetes.
	err = service.CreateKubernetesCluster(ctx, kubeCluster1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a Kubernetes.
	kubeCluster1.Metadata.Description = "description"
	err = service.UpdateKubernetesCluster(ctx, kubeCluster1)
	require.NoError(t, err)
	cluster, err = service.GetKubernetesCluster(ctx, kubeCluster1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(kubeCluster1, cluster, diffopt))

	// Delete a Kubernetes.
	err = service.DeleteKubernetesCluster(ctx, kubeCluster1.GetName())
	require.NoError(t, err)

	expectedAll = []types.KubeCluster{kubeCluster2, kubeCluster3}
	out, err = service.GetKubernetesClusters(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedAll, out, diffopt))

	// Try to delete a Kubernetes that doesn't exist.
	err = service.DeleteKubernetesCluster(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all Kubernetess.
	err = service.DeleteAllKubernetesClusters(ctx)
	require.NoError(t, err)
	out, err = service.GetKubernetesClusters(ctx)
	require.NoError(t, err)
	require.Empty(t, out)
}
