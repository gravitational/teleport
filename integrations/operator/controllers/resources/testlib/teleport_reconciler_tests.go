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

package testlib

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

type ResourceTestingPrimitives[T reconcilers.Resource, K reconcilers.KubernetesCR[T]] interface {
	// Adapter allows to recover the name revision and labels of a resource
	reconcilers.Adapter[T]
	// Setup the testing suite
	Init(setup *TestSetup)
	SetupTeleportFixtures(context.Context) error
	// Interacting with the Teleport Resource
	CreateTeleportResource(context.Context, string) error
	GetTeleportResource(context.Context, string) (T, error)
	DeleteTeleportResource(context.Context, string) error
	// Interacting with the Kubernetes Resource
	CreateKubernetesResource(context.Context, string) error
	DeleteKubernetesResource(context.Context, string) error
	GetKubernetesResource(context.Context, string) (K, error)
	ModifyKubernetesResource(context.Context, string) error
	// Comparing both
	CompareTeleportAndKubernetesResource(T, K) (bool, string)
}

func ResourceCreationTest[T reconcilers.Resource, K reconcilers.KubernetesCR[T]](t *testing.T, test ResourceTestingPrimitives[T, K], opts ...TestOption) {
	ctx := context.Background()
	setup := SetupTestEnv(t, opts...)
	test.Init(setup)
	resourceName := ValidRandomResourceName("resource-")

	err := test.SetupTeleportFixtures(ctx)
	require.NoError(t, err)

	err = test.CreateKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	var tResource T
	FastEventually(t, func() bool {
		tResource, err = test.GetTeleportResource(ctx, resourceName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)

	// We get the kube resource to get the resourceName as it might have been changed if this is a singleton resource
	kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
	require.NoError(t, err)
	require.Equal(t, kubeResource.GetName(), test.GetResourceName(tResource))
	require.Equal(t, types.OriginKubernetes, test.GetResourceOrigin(tResource))

	err = test.DeleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	FastEventually(t, func() bool {
		_, err = test.GetTeleportResource(ctx, resourceName)
		return trace.IsNotFound(err)
	})
}

func ResourceDeletionDriftTest[T reconcilers.Resource, K reconcilers.KubernetesCR[T]](t *testing.T, test ResourceTestingPrimitives[T, K], opts ...TestOption) {
	ctx := context.Background()
	setup := SetupTestEnv(t, opts...)
	test.Init(setup)
	resourceName := ValidRandomResourceName("resource-")

	err := test.SetupTeleportFixtures(ctx)
	require.NoError(t, err)

	err = test.CreateKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	var tResource T
	FastEventually(t, func() bool {
		tResource, err = test.GetTeleportResource(ctx, resourceName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)

	// We get the kube resource to get the resourceName as it might have been changed if this is a singleton resource
	kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
	require.NoError(t, err)
	require.Equal(t, kubeResource.GetName(), test.GetResourceName(tResource))
	require.Equal(t, types.OriginKubernetes, test.GetResourceOrigin(tResource))

	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.StopKubernetesOperator()

	err = test.DeleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
	FastEventually(t, func() bool {
		_, err = test.GetTeleportResource(ctx, resourceName)
		return trace.IsNotFound(err)
	})

	// We flag the resource for deletion in Kubernetes (it won't be fully removed until the operator has processed it and removed the finalizer)
	err = test.DeleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	// Test section: We resume the operator, it should reconcile and recover from the drift
	setup.StartKubernetesOperator(t)

	// The operator should handle the failed Teleport deletion gracefully and unlock the Kubernetes resource deletion
	FastEventually(t, func() bool {
		_, err = test.GetKubernetesResource(ctx, resourceName)
		return kerrors.IsNotFound(err)
	})
}

func ResourceUpdateTest[T reconcilers.Resource, K reconcilers.KubernetesCR[T]](t *testing.T, test ResourceTestingPrimitives[T, K], opts ...TestOption) {
	ctx := context.Background()
	setup := SetupTestEnv(t, opts...)
	test.Init(setup)
	resourceName := ValidRandomResourceName("resource-")

	err := test.SetupTeleportFixtures(ctx)
	require.NoError(t, err)

	// The resource is created in Teleport
	err = test.CreateTeleportResource(ctx, resourceName)
	require.NoError(t, err)

	// The resource is created in Kubernetes, with at least a field altered
	err = test.CreateKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	// Check the resource was updated in Teleport
	FastEventuallyWithT(t, func(c *assert.CollectT) {
		tResource, err := test.GetTeleportResource(ctx, resourceName)
		require.NoError(c, err)

		kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
		require.NoError(c, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.CompareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		assert.True(c, equal)
	})

	// Updating the resource in Kubernetes
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return test.ModifyKubernetesResource(ctx, resourceName)
	})
	require.NoError(t, err)

	// Check the resource was updated in Teleport
	FastEventuallyWithT(t, func(c *assert.CollectT) {
		kubeResource, err := test.GetKubernetesResource(ctx, resourceName)
		require.NoError(c, err)

		tResource, err := test.GetTeleportResource(ctx, resourceName)
		require.NoError(c, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.CompareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		assert.True(c, equal)
	})

	// Delete the resource to avoid leftover state.
	err = test.DeleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
}
