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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	v2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

const teleportUserKind = "TeleportUser"

var (
	teleportUserGVK = schema.GroupVersionKind{
		Group:   v2.GroupVersion.Group,
		Version: v2.GroupVersion.Version,
		Kind:    teleportUserKind,
	}
	userSpec = types.UserSpecV2{
		Roles: []string{"a", "b"},
	}
)

type userTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.User]
}

func (g *userTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *userTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return trace.NewAggregate(
		teleportCreateDummyRole(ctx, "a", g.setup.TeleportClient),
		teleportCreateDummyRole(ctx, "b", g.setup.TeleportClient),
	)
}

func (g *userTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	user, err := types.NewUser(name)
	if err != nil {
		return trace.Wrap(err)
	}
	user.SetOrigin(types.OriginKubernetes)
	user.SetRoles(userSpec.Roles)
	_, err = g.setup.TeleportClient.CreateUser(ctx, user)
	return trace.Wrap(err)
}

func (g *userTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.User, error) {
	return g.setup.TeleportClient.GetUser(ctx, name, false)
}

func (g *userTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteUser(ctx, name))
}

func (g *userTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	user := &v2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: v2.TeleportUserSpec(userSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, user))
}

func (g *userTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	user := &v2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, user))
}

func (g *userTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*v2.TeleportUser, error) {
	user := &v2.TeleportUser{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, user)
	return user, trace.Wrap(err)
}

func (g *userTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	user, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	user.Spec.Traits = wrappers.Traits{
		"foo": []string{"bar", "baz"},
	}
	return g.setup.K8sClient.Update(ctx, user)
}

func (g *userTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.User, kubeResource *v2.TeleportUser) (bool, string) {
	ignoreServerSideDefaults := []cmp.Option{
		cmpopts.IgnoreFields(types.UserSpecV2{}, "CreatedBy"),
		cmpopts.IgnoreFields(types.UserV2{}, "Status"),
	}
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions(ignoreServerSideDefaults...)...)
	return diff == "", diff
}

func TestTeleportUserCreation(t *testing.T) {
	test := &userTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewUserReconciler, test)
}

func TestTeleportUserDeletion(t *testing.T) {
	test := &userTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewUserReconciler, test)
}

func TestTeleportUserDeletionDrift(t *testing.T) {
	test := &userTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewUserReconciler, test)
}

func TestTeleportUserUpdate(t *testing.T) {
	test := &userTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewUserReconciler, test)
}

func TestUserCreationFromYAML(t *testing.T) {
	ctx := context.Background()
	setup := testlib.SetupFakeKubeTestEnv(t)
	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))
	tests := []struct {
		name         string
		userSpecYAML string
		expectedSpec *types.UserSpecV2
	}{
		{
			name: "Valid user without traits",
			userSpecYAML: `
roles:
  - a
`,
			expectedSpec: &types.UserSpecV2{
				Roles: []string{"a"},
			},
		},
		{
			name: "Valid user with trait (list with single element)",
			userSpecYAML: `
roles:
  - a
traits:
  'foo': ['bar']
`,
			expectedSpec: &types.UserSpecV2{
				Roles: []string{"a"},
				Traits: map[string][]string{
					"foo": {"bar"},
				},
			},
		},
		{
			name: "Valid user with traits (list with multiple element)",
			userSpecYAML: `
roles:
  - a
traits:
  'foo': ['bar', 'baz']
`,
			expectedSpec: &types.UserSpecV2{
				Roles: []string{"a"},
				Traits: map[string][]string{
					"foo": {"bar", "baz"},
				},
			},
		},
	}

	for _, tc := range tests {
		// capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Creating the Kubernetes resource. We are using an untyped client to be able to create invalid resources.
			userManifest := map[string]any{}
			err := yaml.Unmarshal([]byte(tc.userSpecYAML), &userManifest)
			require.NoError(t, err)

			userName := validRandomResourceName("user-")

			obj, err := reconcilers.GetUnstructuredObjectFromGVK(teleportUserGVK)
			require.NoError(t, err)
			obj.Object["spec"] = userManifest
			obj.SetName(userName)
			obj.SetNamespace(setup.Namespace.Name)
			err = setup.K8sClient.Create(ctx, obj)
			require.NoError(t, err)

			reconciler, err := resources.NewUserReconciler(setup.K8sClient, setup.TeleportClient)
			require.NoError(t, err)
			// Test execution: Kick off the reconciliation.
			req := reconcile.Request{
				NamespacedName: apimachinerytypes.NamespacedName{
					Namespace: setup.Namespace.Name,
					Name:      userName,
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

			// We wait for Teleport resource creation
			var tUser types.User
			fastEventually(t, func() bool {
				tUser, err = setup.TeleportClient.GetUser(ctx, userName, false /* withSecrets */)
				// If the resource creation should succeed we check the resource was found and validate ownership labels
				return !trace.IsNotFound(err)
			})
			require.NoError(t, err)
			require.Equal(t, userName, tUser.GetName())
			require.Contains(t, tUser.GetMetadata().Labels, types.OriginLabel)
			require.Equal(t, types.OriginKubernetes, tUser.GetMetadata().Labels[types.OriginLabel])
			require.Equal(t, setup.OperatorName, tUser.GetCreatedBy().User.Name)
			expectedUser := &types.UserV2{
				Metadata: types.Metadata{},
				Spec:     *tc.expectedSpec,
			}
			_ = expectedUser.CheckAndSetDefaults()
			compareUserSpecs(t, expectedUser, tUser)
			// Teardown

			user := v2.TeleportUser{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: setup.Namespace.Name,
				},
			}
			require.NoError(t, setup.K8sClient.Delete(ctx, &user))
			_, err = reconciler.Reconcile(ctx, req)
			require.NoError(t, err)
		})
	}
}

func compareUserSpecs(t *testing.T, expectedUser, actualUser types.User) {
	expected, err := teleportResourceToMap(expectedUser)
	require.NoError(t, err)
	actual, err := teleportResourceToMap(actualUser)
	require.NoError(t, err)

	// We don't want compare spec.created_by and metadata as they were tested before and are not 100%
	// managed by the operator
	delete(expected["spec"].(map[string]any), "created_by")
	delete(actual["spec"].(map[string]any), "created_by")

	require.Equal(t, expected["spec"], actual["spec"])
}

func TestUserUpdate(t *testing.T) {
	ctx := context.Background()
	setup := testlib.SetupFakeKubeTestEnv(t)
	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "b", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "x", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "y", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "z", setup.TeleportClient))

	userName := validRandomResourceName("user-")

	// The user does not exist in K8S
	var r v2.TeleportUser
	err := setup.K8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: setup.Namespace.Name,
		Name:      userName,
	}, &r)
	require.True(t, kerrors.IsNotFound(err))

	// The user is created in Teleport
	tUser, err := types.NewUser(userName)
	require.NoError(t, err)
	tUser.SetRoles([]string{"a", "b"})
	metadata := tUser.GetMetadata()
	metadata.Labels = map[string]string{types.OriginLabel: types.OriginKubernetes}
	tUser.SetMetadata(metadata)
	createdBy := types.CreatedBy{
		Connector: nil,
		Time:      time.Now(),
		User: types.UserRef{
			Name: setup.OperatorName,
		},
	}
	tUser.SetCreatedBy(createdBy)

	tUser, err = setup.TeleportClient.CreateUser(ctx, tUser)
	require.NoError(t, err)

	// Wait for the user to enter the cache
	testlib.FastEventually(t, func() bool {
		tUser, err = setup.TeleportClient.GetUser(ctx, tUser.GetName(), false)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	oldRevision := tUser.GetRevision()

	// The user is created in K8S
	k8sUser := v2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: setup.Namespace.Name,
		},
		Spec: v2.TeleportUserSpec{
			Roles: []string{"x", "z"},
		},
	}
	k8sCreateUser(ctx, t, setup.K8sClient, &k8sUser)

	reconciler, err := resources.NewUserReconciler(setup.K8sClient, setup.TeleportClient)
	require.NoError(t, err)
	// Test execution: Kick off the reconciliation.
	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      userName,
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

	// Wait for the new user to enter the cache, else the next reconciliation will cause conflicts.
	testlib.FastEventually(t, func() bool {
		newUser, err := setup.TeleportClient.GetUser(ctx, tUser.GetName(), false)
		assert.NoError(t, err)
		// only proceed when the revision changes
		return newUser.GetRevision() != oldRevision
	})
	require.NoError(t, err)

	// Updating the user in K8S
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	var k8sUserNewVersion v2.TeleportUser
	require.NoError(t,
		setup.K8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: setup.Namespace.Name,
			Name:      userName,
		}, &k8sUserNewVersion),
	)

	k8sUserNewVersion.Spec.Roles = append(k8sUserNewVersion.Spec.Roles, "y")
	require.NoError(t, setup.K8sClient.Update(ctx, &k8sUserNewVersion))

	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	require.Equal(t, setup.OperatorName, tUser.GetCreatedBy().User.Name, "createdBy has not been erased")
}

func k8sCreateUser(ctx context.Context, t *testing.T, kc kclient.Client, user *v2.TeleportUser) {
	err := kc.Create(ctx, user)
	require.NoError(t, err)
}
