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

package resources

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
)

type resourceTestingPrimitives[T TeleportResource, K TeleportKubernetesResource[T]] interface {
	init(setup *testSetup)
	setupTeleportFixtures(context.Context) error
	// Interacting with the Teleport Resource
	createTeleportResource(context.Context, string) error
	getTeleportResource(context.Context, string) (T, error)
	deleteTeleportResource(context.Context, string) error
	// Interacting with the Kubernetes Resource
	createKubernetesResource(context.Context, string) error
	deleteKubernetesResource(context.Context, string) error
	getKubernetesResource(context.Context, string) (K, error)
	modifyKubernetesResource(context.Context, string) error
	// Comparing both
	compareTeleportAndKubernetesResource(T, K) (bool, string)
}

func testResourceCreation[T TeleportResource, K TeleportKubernetesResource[T]](t *testing.T, test resourceTestingPrimitives[T, K]) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	test.init(setup)
	resourceName := validRandomResourceName("resource-")

	err := test.setupTeleportFixtures(ctx)
	require.NoError(t, err)

	err = test.createKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	fastEventually(t, func() bool {
		tResource, err := test.getTeleportResource(ctx, resourceName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tResource.GetName(), resourceName)

		require.Contains(t, tResource.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tResource.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})

	err = test.deleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	fastEventually(t, func() bool {
		_, err = test.getTeleportResource(ctx, resourceName)
		if trace.IsNotFound(err) {
			return true
		}
		require.NoError(t, err)

		return false
	})
}

func testResourceDeletionDrift[T TeleportResource, K TeleportKubernetesResource[T]](t *testing.T, test resourceTestingPrimitives[T, K]) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	test.init(setup)
	resourceName := validRandomResourceName("user-")

	err := test.setupTeleportFixtures(ctx)
	require.NoError(t, err)

	err = test.createKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	fastEventually(t, func() bool {
		tResource, err := test.getTeleportResource(ctx, resourceName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tResource.GetName(), resourceName)

		require.Contains(t, tResource.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tResource.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})
	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.stopKubernetesOperator()

	err = test.deleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
	fastEventually(t, func() bool {
		_, err = test.getTeleportResource(ctx, resourceName)
		return trace.IsNotFound(err)
	})

	// We flag the resource for deletion in Kubernetes (it won't be fully removed until the operator has processed it and removed the finalizer)
	err = test.deleteKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	// Test section: We resume the operator, it should reconcile and recover from the drift
	setup.startKubernetesOperator(t)

	// The operator should handle the failed Teleport deletion gracefully and unlock the Kubernetes resource deletion
	fastEventually(t, func() bool {
		_, err = test.getKubernetesResource(ctx, resourceName)
		return kerrors.IsNotFound(err)
	})
}

func testResourceUpdate[T TeleportResource, K TeleportKubernetesResource[T]](t *testing.T, test resourceTestingPrimitives[T, K]) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	test.init(setup)
	resourceName := validRandomResourceName("user-")

	err := test.setupTeleportFixtures(ctx)
	require.NoError(t, err)

	// The resource is created in Teleport
	err = test.createTeleportResource(ctx, resourceName)
	require.NoError(t, err)

	// The resource is created in Kubernetes, with at least a field altered
	err = test.createKubernetesResource(ctx, resourceName)
	require.NoError(t, err)

	// Check the resource was updated in Teleport
	fastEventually(t, func() bool {
		tResource, err := test.getTeleportResource(ctx, resourceName)
		require.NoError(t, err)

		kubeResource, err := test.getKubernetesResource(ctx, resourceName)
		require.NoError(t, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.compareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		return equal
	})

	// Updating the resource in Kubernetes
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return test.modifyKubernetesResource(ctx, resourceName)
	})
	require.NoError(t, err)

	// Check the resource was updated in Teleport
	fastEventually(t, func() bool {
		kubeResource, err := test.getKubernetesResource(ctx, resourceName)
		require.NoError(t, err)

		tResource, err := test.getTeleportResource(ctx, resourceName)
		require.NoError(t, err)

		// Kubernetes and Teleport resources are in-sync
		equal, diff := test.compareTeleportAndKubernetesResource(tResource, kubeResource)
		if !equal {
			t.Logf("Kubernetes and Teleport resources not sync-ed yet: %s", diff)
		}
		return equal
	})

	// Delete the resource to avoid leftover state.
	err = test.deleteTeleportResource(ctx, resourceName)
	require.NoError(t, err)
}

type FakeResourceWithOrigin types.GithubConnector

type FakeKubernetesResource struct {
	client.Object
}

func (r FakeKubernetesResource) ToTeleport() FakeResourceWithOrigin {
	return nil
}

func (r FakeKubernetesResource) StatusConditions() *[]v1.Condition {
	return nil
}

type FakeKubernetesResourcePtrReceiver struct {
	client.Object
}

func (r *FakeKubernetesResourcePtrReceiver) ToTeleport() FakeResourceWithOrigin {
	return nil
}

func (r *FakeKubernetesResourcePtrReceiver) StatusConditions() *[]v1.Condition {
	return nil
}

func TestNewKubeResource(t *testing.T) {
	// Test with a value receiver
	resource := newKubeResource[FakeResourceWithOrigin, FakeKubernetesResource]()
	require.IsTypef(t, FakeKubernetesResource{}, resource, "Should be of type FakeKubernetesResource")
	require.NotNil(t, resource)

	// Test with a pointer receiver
	resourcePtr := newKubeResource[FakeResourceWithOrigin, *FakeKubernetesResourcePtrReceiver]()
	require.IsTypef(t, &FakeKubernetesResourcePtrReceiver{}, resourcePtr, "Should be a pointer on FakeKubernetesResourcePtrReceiver")
	require.NotNil(t, resourcePtr)
	require.NotNil(t, *resourcePtr)
}
