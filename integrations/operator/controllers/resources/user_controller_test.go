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
	apiresources "github.com/gravitational/teleport/integrations/operator/apis/resources"
	v2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

const teleportUserKind = "TeleportUser"

var teleportUserGVK = schema.GroupVersionKind{
	Group:   v2.GroupVersion.Group,
	Version: v2.GroupVersion.Version,
	Kind:    teleportUserKind,
}

func TestUserCreation(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	userName := validRandomResourceName("user-")

	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "b", setup.TeleportClient))

	// The user is created in K8S
	k8sCreateDummyUser(ctx, t, setup.K8sClient, setup.Namespace.Name, userName)

	var tUser types.User
	var err error
	fastEventually(t, func() bool {
		tUser, err = setup.TeleportClient.GetUser(ctx, userName, false)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	require.Equal(t, userName, tUser.GetName())
	require.Contains(t, tUser.GetMetadata().Labels, types.OriginLabel)
	require.Equal(t, types.OriginKubernetes, tUser.GetMetadata().Labels[types.OriginLabel])

	// The user is deleted in K8S
	k8sDeleteUser(ctx, t, setup.K8sClient, userName, setup.Namespace.Name)

	fastEventually(t, func() bool {
		_, err := setup.TeleportClient.GetUser(ctx, userName, false)
		return trace.IsNotFound(err)
	})
}

func TestUserCreationFromYAML(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))
	tests := []struct {
		name         string
		userSpecYAML string
		shouldFail   bool
		expectedSpec *types.UserSpecV2
	}{
		{
			name: "Valid user without traits",
			userSpecYAML: `
roles:
  - a
`,
			shouldFail: false,
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
			shouldFail: false,
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
			shouldFail: false,
			expectedSpec: &types.UserSpecV2{
				Roles: []string{"a"},
				Traits: map[string][]string{
					"foo": {"bar", "baz"},
				},
			},
		},
		{
			name: "Invalid user with non-existing role",
			userSpecYAML: `
roles:
  - does-not-exist
traits:
  'foo': ['bar', 'baz']
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

			// If failure is expected we should not see the resource in Teleport
			if tc.shouldFail {
				fastEventually(t, func() bool {
					// We check status.Conditions was updated, this means the reconciliation happened
					_ = setup.K8sClient.Get(ctx, kclient.ObjectKey{
						Namespace: setup.Namespace.Name,
						Name:      userName,
					}, obj)
					errorConditions := getUserStatusConditionError(obj.Object)
					// If there's no error condition, reconciliation has not happened yet
					return len(errorConditions) != 0
				})
				_, err = setup.TeleportClient.GetUser(ctx, userName, false /* withSecrets */)
				require.True(t, trace.IsNotFound(err), "The user should not be created in Teleport")
			} else {
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
			}
			// Teardown

			// The role is deleted in K8S
			k8sDeleteUser(ctx, t, setup.K8sClient, userName, setup.Namespace.Name)

			// We wait for the role deletion in Teleport
			fastEventually(t, func() bool {
				_, err := setup.TeleportClient.GetUser(ctx, userName, false /* withSecrets */)
				return trace.IsNotFound(err)
			})
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

// TestUserDeletionDrift tests how the Kubernetes operator reacts when it is asked to delete a user that was
// already deleted in Teleport
func TestUserDeletionDrift(t *testing.T) {
	// Setup section: start the operator, and create a user
	ctx := context.Background()
	setup := setupTestEnv(t)
	userName := validRandomResourceName("user-")

	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))
	require.NoError(t, teleportCreateDummyRole(ctx, "b", setup.TeleportClient))

	// The user is created in K8S
	k8sCreateDummyUser(ctx, t, setup.K8sClient, setup.Namespace.Name, userName)

	var tUser types.User
	var err error
	fastEventually(t, func() bool {
		tUser, err = setup.TeleportClient.GetUser(ctx, userName, false)
		return !trace.IsNotFound(err)
	})
	require.NoError(t, err)
	require.Equal(t, userName, tUser.GetName())
	require.Contains(t, tUser.GetMetadata().Labels, types.OriginLabel)
	require.Equal(t, types.OriginKubernetes, tUser.GetMetadata().Labels[types.OriginLabel])

	// We cause a drift by altering the Teleport resource.
	// To make sure the operator does not reconcile while we're finished we suspend the operator
	setup.StopKubernetesOperator()

	err = setup.TeleportClient.DeleteUser(ctx, userName)
	require.NoError(t, err)
	fastEventually(t, func() bool {
		_, err := setup.TeleportClient.GetUser(ctx, userName, false)
		return trace.IsNotFound(err)
	})

	// We flag the role for deletion in Kubernetes (it won't be fully removed until the operator has processed it and removed the finalizer)
	k8sDeleteUser(ctx, t, setup.K8sClient, userName, setup.Namespace.Name)

	// Test section: We resume the operator, it should reconcile and recover from the drift
	setup.StartKubernetesOperator(t)

	// The operator should handle the failed Teleport deletion gracefully and unlock the Kubernetes resource deletion
	var k8sUser v2.TeleportUser
	fastEventually(t, func() bool {
		err = setup.K8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: setup.Namespace.Name,
			Name:      userName,
		}, &k8sUser)
		return kerrors.IsNotFound(err)
	})
}

func TestUserUpdate(t *testing.T) {
	ctx := context.Background()
	setup := setupTestEnv(t)
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

	// The user is updated in Teleport
	fastEventually(t, func() bool {
		tUser, err := setup.TeleportClient.GetUser(ctx, userName, false)
		assert.NoError(t, err)

		// TeleportUser was updated with new roles
		return compareRoles([]string{"x", "z"}, tUser.GetRoles())
	})

	// Updating the user in K8S
	// The modification can fail because of a conflict with the resource controller. We retry if that happens.
	var k8sUserNewVersion v2.TeleportUser
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := setup.K8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: setup.Namespace.Name,
			Name:      userName,
		}, &k8sUserNewVersion)
		if err != nil {
			return err
		}

		k8sUserNewVersion.Spec.Roles = append(k8sUserNewVersion.Spec.Roles, "y")
		return setup.K8sClient.Update(ctx, &k8sUserNewVersion)
	})
	require.NoError(t, err)

	// Updates the user in Teleport
	fastEventuallyWithT(t, func(c *assert.CollectT) {
		tUser, err := setup.TeleportClient.GetUser(ctx, userName, false)
		require.NoError(c, err)

		// TeleportUser updated with new roles
		assert.ElementsMatch(c, tUser.GetRoles(), []string{"x", "z", "y"})
	})
	require.Equal(t, setup.OperatorName, tUser.GetCreatedBy().User.Name, "createdBy has not been erased")
}

func k8sCreateDummyUser(ctx context.Context, t *testing.T, kc kclient.Client, namespace, userName string) {
	user := v2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
		Spec: v2.TeleportUserSpec{
			Roles: []string{"a", "b"},
		},
	}
	k8sCreateUser(ctx, t, kc, &user)
}

func k8sDeleteUser(ctx context.Context, t *testing.T, kc kclient.Client, userName, namespace string) {
	user := v2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
	}
	err := kc.Delete(ctx, &user)
	require.NoError(t, err)
}

func k8sCreateUser(ctx context.Context, t *testing.T, kc kclient.Client, user *v2.TeleportUser) {
	err := kc.Create(ctx, user)
	require.NoError(t, err)
}

func getUserStatusConditionError(object map[string]any) []metav1.Condition {
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

func compareRoles(expected, actual []string) bool {
	opts := testlib.CompareOptions(cmpopts.SortSlices(func(a, b string) bool { return a < b }))
	return cmp.Diff(
		expected,
		actual,
		opts...,
	) == ""
}
