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
	"reflect"
	"sort"
	"testing"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
	"github.com/gravitational/teleport/operator/controllers/resources"
)

// When I create or delete a TeleportRole CR in Kubernetes,
// the corresponding TeleportRole must be created/deleted in Teleport.
func TestRoleCreation(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	roleName := validRandomResourceName("role-")

	// End of setup, we create the role in Kubernetes
	k8sCreateDummyRole(ctx, t, setup.K8sClient, setup.Namespace.Name, roleName)

	// We wait for the role to be created in Teleport
	fastEventually(t, func() bool {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		// Role should have the same name, and have the Kubernetes origin label
		require.Equal(t, tRole.GetName(), roleName)
		require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})

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
			name: "Valid login list",
			roleSpecYAML: `
allow:
  logins:
  - ubuntu
  - root
`,
			shouldFail: false,
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"ubuntu", "root"},
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
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Creating the Kubernetes resource. We are using an untyped client to be able to create invalid resources.
			roleManifest := map[string]interface{}{}
			err := yaml.Unmarshal([]byte(tc.roleSpecYAML), &roleManifest)
			require.NoError(t, err)

			roleName := validRandomResourceName("role-")

			obj := resources.GetUnstructuredObjectFromGVK(resources.TeleportRoleGVKV5)
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
					if len(errorConditions) == 0 {
						return false
					}

					_, err := setup.TeleportClient.GetRole(ctx, roleName)
					require.True(t, trace.IsNotFound(err), "The role should not be created in Teleport")
					return true
				})
			} else {
				// We wait for Teleport resource creation
				fastEventually(t, func() bool {
					tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
					// If the resource creation should succeed we check the resource was found and validate ownership labels
					if trace.IsNotFound(err) {
						return false
					}
					require.NoError(t, err)

					require.Equal(t, tRole.GetName(), roleName)
					require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
					require.Equal(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)
					expectedRole, _ := types.NewRole(roleName, *tc.expectedSpec)
					compareRoleSpecs(t, expectedRole, tRole)

					return true
				})
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

	fastEventually(t, func() bool {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tRole.GetName(), roleName)

		require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})
	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.StopKubernetesOperator()

	err := setup.TeleportClient.DeleteRole(ctx, roleName)
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
	fastEventually(t, func() bool {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		return reflect.DeepEqual(logins, []string{"x", "z"})
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
	fastEventually(t, func() bool {
		tRole, err := setup.TeleportClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		return reflect.DeepEqual(logins, []string{"admin", "root", "x", "z"})
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

func TestAddTeleportResourceOriginRole(t *testing.T) {
	r := resources.RoleReconciler{}
	tests := []struct {
		name     string
		resource types.Role
	}{
		{
			name: "origin already set correctly",
			resource: &types.RoleV6{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
		},
		{
			name: "origin already set incorrectly",
			resource: &types.RoleV6{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
		},
		{
			name: "origin not set",
			resource: &types.RoleV6{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{"foo": "bar"},
				},
			},
		},
		{
			name: "no labels",
			resource: &types.RoleV6{
				Metadata: types.Metadata{
					Name: "user with no labels",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r.AddTeleportResourceOrigin(tc.resource)
			metadata := tc.resource.GetMetadata()
			require.Contains(t, metadata.Labels, types.OriginLabel)
			require.Equal(t, metadata.Labels[types.OriginLabel], types.OriginKubernetes)
		})
	}
}

func getRoleStatusConditionError(object map[string]interface{}) []metav1.Condition {
	var conditionsWithError []metav1.Condition
	var status resourcesv5.TeleportRoleStatus
	_ = mapstructure.Decode(object["status"], &status)

	for _, condition := range status.Conditions {
		if condition.Status == metav1.ConditionFalse {
			conditionsWithError = append(conditionsWithError, condition)
		}
	}
	return conditionsWithError
}
