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

package resources_test

import (
	"context"
	"sort"
	"testing"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	apiresources "github.com/gravitational/teleport/integrations/operator/apis/resources"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

var TeleportRoleGVKV5 = schema.GroupVersionKind{
	Group:   resourcesv5.GroupVersion.Group,
	Version: resourcesv5.GroupVersion.Version,
	Kind:    "TeleportRole",
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

func TestRoleCreationFromYAML(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	tests := []struct {
		name         string
		roleSpecYAML string
		shouldFail   bool
		expectedSpec *types.RoleSpecV6
	}{
		{
			name: "Valid login list with integer create_host_user_mode",
			roleSpecYAML: `
allow:
  logins:
  - ubuntu
  - root
options:
  create_host_user_mode: 3
`,
			shouldFail: false,
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"ubuntu", "root"},
				},
				Options: types.RoleOptions{
					CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
				},
			},
		},
		{
			name: "Valid login list",
			roleSpecYAML: `
allow:
  logins:
  - ubuntu
  - root
options:
  create_host_user_mode: keep
`,
			shouldFail: false,
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"ubuntu", "root"},
				},
				Options: types.RoleOptions{
					CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
				},
			},
		},
		{
			name: "Valid node_labels wildcard (list version)",
			roleSpecYAML: `
allow:
  node_labels:
    '*': ['*']
`,
			shouldFail: false,
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					NodeLabels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
		{
			name: "Valid node_labels wildcard (string version)",
			roleSpecYAML: `
allow:
  node_labels:
    '*': '*'
`,
			shouldFail: false,
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					NodeLabels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
		{
			name: "Invalid node_labels (label value is integer)",
			roleSpecYAML: `
allow:
  node_labels:
    'foo': 1
`,
			shouldFail:   true,
			expectedSpec: nil,
		},
		{
			name: "Invalid node_labels (label value is object)",
			roleSpecYAML: `
allow:
  node_labels:
    'foo':
      'bar': 'baz'
    'logins':
      - 'ubuntu'
`,
			shouldFail:   true,
			expectedSpec: nil,
		},
	}

	for _, tc := range tests {
		// capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Creating the Kubernetes resource. We are using an untyped client to be able to create invalid resources.
			roleManifest := map[string]any{}
			err := yaml.Unmarshal([]byte(tc.roleSpecYAML), &roleManifest)
			require.NoError(t, err)

			roleName := validRandomResourceName("role-")

			obj, err := reconcilers.GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
			require.NoError(t, err)
			obj.Object["spec"] = roleManifest
			obj.SetName(roleName)
			obj.SetNamespace(setup.Namespace.Name)
			err = setup.K8sClient.Create(ctx, obj)
			require.NoError(t, err)

			// If failure is expected we should not see the resource in Teleport
			if tc.shouldFail {
				fastEventually(t, func() bool {
					// We check status.Conditions was updated, this means the reconciliation happened
					_ = setup.K8sClient.Get(ctx, kclient.ObjectKey{
						Namespace: setup.Namespace.Name,
						Name:      roleName,
					}, obj)
					errorConditions := getRoleStatusConditionError(obj.Object)
					// If there's no error condition, reconciliation has not happened yet
					return len(errorConditions) != 0
				})
				_, err = setup.TeleportClient.GetRole(ctx, roleName)
				require.True(t, trace.IsNotFound(err), "The role should not be created in Teleport")
			} else {
				var tRole types.Role
				// We wait for Teleport resource creation
				fastEventually(t, func() bool {
					tRole, err = setup.TeleportClient.GetRole(ctx, roleName)
					return !trace.IsNotFound(err)
				})
				// If the resource creation should succeed we check the resource was found and validate ownership labels
				require.NoError(t, err)
				require.Equal(t, roleName, tRole.GetName())
				require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
				require.Equal(t, types.OriginKubernetes, tRole.GetMetadata().Labels[types.OriginLabel])
				expectedRole, _ := types.NewRoleWithVersion(roleName, types.V7, *tc.expectedSpec)
				compareRoleSpecs(t, expectedRole, tRole)
			}
			// Teardown

			// The role is deleted in K8S
			k8sDeleteRole(ctx, t, setup.K8sClient, roleName, setup.Namespace.Name)

			// We wait for the role deletion in Teleport
			fastEventually(t, func() bool {
				_, err := setup.TeleportClient.GetRole(ctx, roleName)
				return trace.IsNotFound(err)
			})
		})
	}
}

func compareRoleSpecs(t *testing.T, expectedRole, actualRole types.Role) {
	expected, err := teleportResourceToMap(expectedRole)
	require.NoError(t, err)
	actual, err := teleportResourceToMap(actualRole)
	require.NoError(t, err)

	require.Equal(t, expected["spec"], actual["spec"])
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

func getRoleStatusConditionError(object map[string]any) []metav1.Condition {
	var conditionsWithError []metav1.Condition
	var status apiresources.Status
	_ = mapstructure.Decode(object["status"], &status)

	for _, condition := range status.Conditions {
		if condition.Status == metav1.ConditionFalse {
			conditionsWithError = append(conditionsWithError, condition)
		}
	}
	return conditionsWithError
}
