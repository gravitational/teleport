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

package resources_test

import (
	"context"
	"sort"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
)

const teleportRoleKind = "TeleportRole"

// TODO(for v12): Have the Role controller to use the generic Teleport reconciler
// This means we'll have to move back to a statically typed client.
// This will require removing the crdgen hack, fixing TeleportRole JSON serialization

var TeleportRoleGVKV5 = schema.GroupVersionKind{
	Group:   resourcesv5.GroupVersion.Group,
	Version: resourcesv5.GroupVersion.Version,
	Kind:    teleportRoleKind,
}

// When I create or delete a TeleportRole CR in Kubernetes,
// the corresponding TeleportRole must be created/deleted in Teleport.
func TestRoleCreation(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	roleName := validRandomResourceName("role-")

	// End of setup, we create the role in Kubernetes
	k8sCreateDummyRole(ctx, t, setup.K8sClient, setup.Namespace.Name, roleName)

	var tRole types.Role
	var err error
	// We wait for the role to be created in Teleport
	fastEventually(t, func() bool {
		tRole, err = setup.TeleportClient.GetRole(ctx, roleName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)

	// Role should have the same name, and have the Kubernetes origin label
	require.Equal(t, roleName, tRole.GetName())
	require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
	require.Equal(t, types.OriginKubernetes, tRole.GetMetadata().Labels[types.OriginLabel])

	// Cleanup and setup, we delete the role in Kubernetes
	k8sDeleteRole(ctx, t, setup.K8sClient, roleName, setup.Namespace.Name)

	// We wait for the role to be deleted in Teleport
	fastEventually(t, func() bool {
		_, err := setup.TeleportClient.GetRole(ctx, roleName)
		return trace.IsNotFound(err)
	})
}

// TestRoleDeletionDrift tests how the Kubernetes operator reacts when it is asked to delete a role that was
// already deleted in Teleport
func TestRoleDeletionDrift(t *testing.T) {
	// Setup section: start the operator, and create a role
	ctx := context.Background()
	setup := setupTestEnv(t)
	roleName := validRandomResourceName("role-")

	// The role is created in K8S
	k8sCreateDummyRole(ctx, t, setup.K8sClient, setup.Namespace.Name, roleName)

	var tRole types.Role
	var err error
	fastEventually(t, func() bool {
		tRole, err = setup.TeleportClient.GetRole(ctx, roleName)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	require.Equal(t, roleName, tRole.GetName())
	require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
	require.Equal(t, types.OriginKubernetes, tRole.GetMetadata().Labels[types.OriginLabel])

	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.StopKubernetesOperator()

	err = setup.TeleportClient.DeleteRole(ctx, roleName)
	require.NoError(t, err)
	fastEventually(t, func() bool {
		_, err := setup.TeleportClient.GetRole(ctx, roleName)
		return trace.IsNotFound(err)
	})

	// We flag the role for deletion in Kubernetes (it won't be fully remopved until the operator has processed it and removed the finalizer)
	k8sDeleteRole(ctx, t, setup.K8sClient, roleName, setup.Namespace.Name)

	// Test section: We resume the operator, it should reconcile and recover from the drift
	setup.StartKubernetesOperator(t)

	// The operator should handle the failed Teleport deletion gracefully and unlock the Kubernetes resource deletion
	var k8sRole resourcesv5.TeleportRole
	fastEventually(t, func() bool {
		err = setup.K8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: setup.Namespace.Name,
			Name:      roleName,
		}, &k8sRole)
		return kerrors.IsNotFound(err)
	})
}

func TestRoleUpdate(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	roleName := validRandomResourceName("role-")

	// The role does not exist in K8S
	var r resourcesv5.TeleportRole
	err := setup.K8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: setup.Namespace.Name,
		Name:      roleName,
	}, &r)
	require.True(t, kerrors.IsNotFound(err))

	require.NoError(t, teleportCreateDummyRole(ctx, roleName, setup.TeleportClient))

	// The role is created in K8S
	k8sRole := resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv5.TeleportRoleSpec{
			Allow: types.RoleConditions{
				Logins: []string{"x", "z"},
			},
		},
	}
	k8sCreateRole(ctx, t, setup.K8sClient, &k8sRole)

	// The role is updated in Teleport
	fastEventuallyWithT(t, func(c *assert.CollectT) {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		require.NoError(c, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		assert.ElementsMatch(c, logins, []string{"x", "z"})
	})

	// Updating the role in K8S
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	var k8sRoleNewVersion resourcesv5.TeleportRole
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := setup.K8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: setup.Namespace.Name,
			Name:      roleName,
		}, &k8sRoleNewVersion)
		if err != nil {
			return err
		}

		k8sRoleNewVersion.Spec.Allow.Logins = append(k8sRoleNewVersion.Spec.Allow.Logins, "admin", "root")
		return setup.K8sClient.Update(ctx, &k8sRoleNewVersion)
	})
	require.NoError(t, err)

	// Updates the role in Teleport
	fastEventuallyWithT(t, func(c *assert.CollectT) {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		require.NoError(c, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		assert.ElementsMatch(c, logins, []string{"admin", "root", "x", "z"})
	})
}

func k8sCreateDummyRole(ctx context.Context, t *testing.T, kc kclient.Client, namespace, roleName string) {
	role := resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Spec: resourcesv5.TeleportRoleSpec{
			Allow: types.RoleConditions{
				Logins: []string{"a", "b"},
			},
		},
	}
	k8sCreateRole(ctx, t, kc, &role)
}

func k8sDeleteRole(ctx context.Context, t *testing.T, kc kclient.Client, roleName, namespace string) {
	role := resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
	}
	err := kc.Delete(ctx, &role)
	require.NoError(t, err)
}

func k8sCreateRole(ctx context.Context, t *testing.T, kc kclient.Client, role *resourcesv5.TeleportRole) {
	err := kc.Create(ctx, role)
	require.NoError(t, err)
}
