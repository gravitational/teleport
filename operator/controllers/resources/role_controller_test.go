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
	"encoding/json"
	apiutils "github.com/gravitational/teleport/api/utils"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
)

// When I create a TeleportRole CR in Kubernetes, the corresponding TeleportRole must be created in Teleport
// When I delete a TeleportRole CR in Kubernetes, the corresponding TeleportRole must be deleted in Teleport
func TestRoleCreation(t *testing.T) {
	ctx, tClient, k8sClient, ns := setupKubernetesAndTeleport(t)
	roleName := validRandomResourceName("role-")

	// End of setup, we create the role in Kubernetes
	k8sCreateDummyRole(ctx, t, k8sClient, ns.Name, roleName)

	// We wait for the role to be created in Teleport
	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
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

	// Cleanup and test, we delete the role in Kubernetes```
	k8sDeleteRole(ctx, t, k8sClient, roleName, ns.Name)

	// We wait for the role to be deleted in Teleport
	fastEventually(t, func() bool {
		_, err := tClient.GetRole(ctx, roleName)
		return trace.IsNotFound(err)
	})
}

func TestRoleCreationFromYAML(t *testing.T) {
	ctx, tClient, k8sClient, ns := setupKubernetesAndTeleport(t)
	tests := []struct {
		name         string
		roleSpecYAML []byte
		shouldFail   bool
		expectedSpec *types.RoleSpecV5
	}{
		{
			"Valid login list",
			[]byte(`
allow:
  logins:
  - ubuntu
  - root
`),
			false,
			&types.RoleSpecV5{
				Allow: types.RoleConditions{
					Logins: []string{"ubuntu", "root"},
				},
			},
		},
		{
			"Valid node_labels wildcard (list version)",
			[]byte(`
allow:
  node_labels:
    '*': ['*']
`),
			false,
			&types.RoleSpecV5{
				Allow: types.RoleConditions{
					NodeLabels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
		{
			"Valid node_labels wildcard (string version)",
			[]byte(`
allow:
  node_labels:
    '*': '*'
`),
			false,
			&types.RoleSpecV5{
				Allow: types.RoleConditions{
					NodeLabels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
		{
			"Invalid node_labels (label value is integer)",
			[]byte(`
allow:
  node_labels:
    'foo': 1
`),
			true,
			nil,
		},
		{
			"Invalid node_labels (label value is object)",
			[]byte(`
allow:
  node_labels:
    'foo':
      'bar': 'baz'
    'logins':
      - 'ubuntu'
`),
			true,
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Creating the Kubernetes resource. We are using an untyped client to be able to create invalid resources.
			roleManifest := map[string]interface{}{}
			err := yaml.Unmarshal(tc.roleSpecYAML, &roleManifest)
			require.NoError(t, err)

			roleName := validRandomResourceName("role-")

			obj := getUnstructuredObjectFromGVK(TeleportRoleGVK)
			obj.Object["spec"] = roleManifest
			obj.SetName(roleName)
			obj.SetNamespace(ns.Name)
			err = k8sClient.Create(ctx, obj)
			require.NoError(t, err)

			// If failure is expected we should not see the resource in Teleport
			if tc.shouldFail {
				// We wait 1 second to ensure reconciliation happened
				time.Sleep(time.Second)
				require.True(t, trace.IsNotFound(err), "The role should not be created in Teleport")
			} else {
				// We wait for Teleport resource creation
				fastEventually(t, func() bool {
					tRole, err := tClient.GetRole(ctx, roleName)
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
			k8sDeleteRole(ctx, t, k8sClient, roleName, ns.Name)

			// We wait for the role deletion in Teleport
			fastEventually(t, func() bool {
				_, err := tClient.GetRole(ctx, roleName)
				return trace.IsNotFound(err)
			})
		})
	}
}

func compareRoleSpecs(t *testing.T, expectedRole, actualRole types.Role) {
	expectedJSON, _ := json.Marshal(expectedRole)
	expected := make(map[string]interface{})
	_ = json.Unmarshal(expectedJSON, &expected)
	actualJSON, _ := json.Marshal(actualRole)
	actual := make(map[string]interface{})
	_ = json.Unmarshal(actualJSON, &actual)

	require.Equal(t, expected["spec"], actual["spec"])
}

func TestRoleDeletionDrift(t *testing.T) {
	ctx, tClient, k8sClient, ns := setupKubernetesAndTeleport(t)
	roleName := validRandomResourceName("role-")

	// The role is created in K8S
	k8sCreateDummyRole(ctx, t, k8sClient, ns.Name, roleName)

	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tRole.GetName(), roleName)

		require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})

	err := tClient.DeleteRole(ctx, roleName)
	require.NoError(t, err)
	fastEventually(t, func() bool {
		_, err := tClient.GetRole(ctx, roleName)
		return trace.IsNotFound(err)
	})

	// The role is deleted in K8S
	k8sDeleteRole(ctx, t, k8sClient, roleName, ns.Name)

	var k8sRole resourcesv5.TeleportRole
	fastEventually(t, func() bool {
		err = k8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: ns.Name,
			Name:      roleName,
		}, &k8sRole)
		return kerrors.IsNotFound(err)
	})
}

func TestRoleUpdate(t *testing.T) {
	ctx, tClient, k8sClient, ns := setupKubernetesAndTeleport(t)
	roleName := validRandomResourceName("role-")

	// The role does not exist in K8S
	var r resourcesv5.TeleportRole
	err := k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
		Name:      roleName,
	}, &r)
	require.True(t, kerrors.IsNotFound(err))

	err = teleportCreateDummyRole(ctx, t, roleName, tClient)
	require.NoError(t, err)

	// The role is created in K8S
	k8sRole := resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns.Name,
		},
		Spec: resourcesv5.TeleportRoleSpec{
			Allow: types.RoleConditions{
				Logins: []string{"x", "z"},
			},
		},
	}
	k8sCreateRole(ctx, t, k8sClient, &k8sRole)

	// The role is updated in Teleport
	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		return reflect.DeepEqual(logins, []string{"x", "z"})
	})

	// Updating the role in K8S
	var k8sRoleNewVersion resourcesv5.TeleportRole
	err = k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
		Name:      roleName,
	}, &k8sRoleNewVersion)
	require.NoError(t, err)

	k8sRoleNewVersion.Spec.Allow.Logins = append(k8sRoleNewVersion.Spec.Allow.Logins, "admin", "root")
	err = k8sClient.Update(ctx, &k8sRoleNewVersion)
	require.NoError(t, err)

	// Updates the role in Teleport
	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// TeleportRole updated with new logins
		logins := tRole.GetLogins(types.Allow)
		sort.Strings(logins)
		return reflect.DeepEqual(logins, []string{"admin", "root", "x", "z"})
	})
}

func setupKubernetesAndTeleport(t *testing.T) (context.Context, auth.ClientI, kclient.Client, *v1.Namespace) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	ns := createNamespaceForTest(t, k8sClient)
	return ctx, tClient, k8sClient, ns
}

func teleportCreateDummyRole(ctx context.Context, t *testing.T, roleName string, tClient auth.ClientI) error {
	// The role is created in Teleport
	tRole, err := types.NewRole(roleName, types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"a", "b"},
		},
	})
	require.NoError(t, err)
	metadata := tRole.GetMetadata()
	metadata.Labels = map[string]string{types.OriginLabel: types.OriginKubernetes}
	tRole.SetMetadata(metadata)

	err = tClient.UpsertRole(ctx, tRole)
	require.NoError(t, err)
	return err
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
	r := RoleReconciler{}
	tests := []struct {
		name     string
		resource types.Role
	}{
		{
			name: "origin already set correctly",
			resource: &types.RoleV5{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
		},
		{
			name: "origin already set incorrectly",
			resource: &types.RoleV5{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
		},
		{
			name: "origin not set",
			resource: &types.RoleV5{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{"foo": "bar"},
				},
			},
		},
		{
			name: "no labels",
			resource: &types.RoleV5{
				Metadata: types.Metadata{
					Name: "user with no labels",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r.addTeleportResourceOrigin(tc.resource)
			metadata := tc.resource.GetMetadata()
			require.Contains(t, metadata.Labels, types.OriginLabel)
			require.Equal(t, metadata.Labels[types.OriginLabel], types.OriginKubernetes)
		})
	}
}
