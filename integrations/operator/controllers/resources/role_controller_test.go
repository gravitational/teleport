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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var TeleportRoleGVKV5 = schema.GroupVersionKind{
	Group:   resourcesv5.GroupVersion.Group,
	Version: resourcesv5.GroupVersion.Version,
	Kind:    "TeleportRole",
}

func TestRoleCreationFromYAML(t *testing.T) {
	ctx := context.Background()
	setup := testlib.SetupFakeKubeTestEnv(t)
	tests := []struct {
		name         string
		roleSpecYAML string
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
			expectedSpec: &types.RoleSpecV6{
				Allow: types.RoleConditions{
					NodeLabels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
	}

	reconciler, err := resources.NewRoleReconciler(setup.K8sClient, setup.TeleportClient)
	require.NoError(t, err)

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

			// Test execution: Kick off the reconciliation.
			req := reconcile.Request{
				NamespacedName: apimachinerytypes.NamespacedName{
					Namespace: setup.Namespace.Name,
					Name:      roleName,
				},
			}
			// First reconciliation should set the finalizer and exit.
			_, err = reconciler.Reconcile(ctx, req)
			require.NoError(t, err)
			// Second reconciliation should create the Teleport resource.
			// In a real cluster we should receive the event of our own finalizer change
			// and this wakes us for a second round.
			_, err = reconciler.Reconcile(ctx, req)
			require.NoError(t, err)

			var tRole types.Role
			// We wait for the Teleport cache.
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
			// Teardown

			// The role is deleted in K8S
			role := resourcesv5.TeleportRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: setup.Namespace.Name,
				},
			}
			require.NoError(t, setup.K8sClient.Delete(ctx, &role))
			_, err = reconciler.Reconcile(ctx, req)
			require.NoError(t, err)
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

var roleV5Spec = types.RoleSpecV6{
	Options: types.RoleOptions{
		ForwardAgent: true,
	},
	Allow: types.RoleConditions{
		Logins:           []string{"foo"},
		KubernetesLabels: types.Labels{"env": {"dev", "prod"}},
		KubernetesResources: []types.KubernetesResource{
			{
				Kind:      "pod",
				Namespace: "monitoring",
				Name:      "^prometheus-.*",
			},
		},
	},
	Deny: types.RoleConditions{},
}

type roleTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.Role]
}

func (g *roleTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *roleTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *roleTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	role, err := types.NewRoleWithVersion(name, types.V5, roleV5Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	role.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.CreateRole(ctx, role)
	return trace.Wrap(err)
}

func (g *roleTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Role, error) {
	return g.setup.TeleportClient.GetRole(ctx, name)
}

func (g *roleTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteRole(ctx, name))
}

func (g *roleTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv5.TeleportRoleSpec(roleV5Spec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, role))
}

func (g *roleTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv5.TeleportRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, role))
}

func (g *roleTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv5.TeleportRole, error) {
	role := &resourcesv5.TeleportRole{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, role)
	return role, trace.Wrap(err)
}

func (g *roleTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	role, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	role.Spec.Allow.Logins = []string{"foo", "bar"}
	return g.setup.K8sClient.Update(ctx, role)
}

func (g *roleTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Role, kubeResource *resourcesv5.TeleportRole) (bool, string) {
	ignoreServerSideDefaults := []cmp.Option{
		cmpopts.IgnoreFields(types.RoleSpecV6{}, "Options"),
		cmpopts.IgnoreFields(types.RoleConditions{}, "Namespaces"),
	}
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions(ignoreServerSideDefaults...)...)
	return diff == "", diff
}

func TestTeleportRoleCreation(t *testing.T) {
	test := &roleTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewRoleReconciler, test)
}

func TestTeleportRoleDeletion(t *testing.T) {
	test := &roleTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewRoleReconciler, test)
}

func TestTeleportRoleDeletionDrift(t *testing.T) {
	test := &roleTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewRoleReconciler, test)
}

func TestTeleportRoleUpdate(t *testing.T) {
	test := &roleTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewRoleReconciler, test)
}
