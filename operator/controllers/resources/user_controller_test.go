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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	"github.com/gravitational/trace"
)

func TestUserCreation(t *testing.T) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	ns := createNamespaceForTest(t, k8sClient)
	userName := validRandomResourceName("user-")

	teleportCreateDummyRole(ctx, t, "a", tClient)
	teleportCreateDummyRole(ctx, t, "b", tClient)
	// The user is created in K8S
	k8sCreateDummyUser(ctx, t, k8sClient, ns.Name, userName)

	fastEventually(t, func() bool {
		tUser, err := tClient.GetUser(userName, false)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tUser.GetName(), userName)

		require.Contains(t, tUser.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tUser.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})

	// The user is deleted in K8S
	k8sDeleteUser(ctx, t, k8sClient, userName, ns.Name)

	fastEventually(t, func() bool {
		_, err := tClient.GetUser(userName, false)
		return trace.IsNotFound(err)
	})
}

func TestUserDeletionDrift(t *testing.T) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	ns := createNamespaceForTest(t, k8sClient)
	userName := validRandomResourceName("user-")

	teleportCreateDummyRole(ctx, t, "a", tClient)
	teleportCreateDummyRole(ctx, t, "b", tClient)

	// The user is created in K8S
	k8sCreateDummyUser(ctx, t, k8sClient, ns.Name, userName)

	fastEventually(t, func() bool {
		tUser, err := tClient.GetUser(userName, false)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tUser.GetName(), userName)

		require.Contains(t, tUser.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tUser.GetMetadata().Labels[types.OriginLabel], types.OriginKubernetes)

		return true
	})

	err := tClient.DeleteUser(ctx, userName)
	require.NoError(t, err)
	fastEventually(t, func() bool {
		_, err := tClient.GetUser(userName, false)
		return trace.IsNotFound(err)
	})

	// The role is deleted in K8S
	k8sDeleteUser(ctx, t, k8sClient, userName, ns.Name)

	var k8sUser resourcesv2.TeleportUser
	fastEventually(t, func() bool {
		err = k8sClient.Get(ctx, kclient.ObjectKey{
			Namespace: ns.Name,
			Name:      userName,
		}, &k8sUser)
		return kerrors.IsNotFound(err)
	})
}

func TestUserUpdate(t *testing.T) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	teleportCreateDummyRole(ctx, t, "a", tClient)
	teleportCreateDummyRole(ctx, t, "b", tClient)
	teleportCreateDummyRole(ctx, t, "x", tClient)
	teleportCreateDummyRole(ctx, t, "y", tClient)
	teleportCreateDummyRole(ctx, t, "z", tClient)

	ns := createNamespaceForTest(t, k8sClient)
	userName := validRandomResourceName("user-")

	// The user does not exist in K8S
	var r resourcesv2.TeleportUser
	err := k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
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

	err = tClient.CreateUser(ctx, tUser)
	require.NoError(t, err)

	// The user is created in K8S
	k8sUser := resourcesv2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: ns.Name,
		},
		Spec: resourcesv2.TeleportUserSpec{
			Roles: []string{"x", "z"},
		},
	}
	k8sCreateUser(ctx, t, k8sClient, &k8sUser)

	// The user is updated in Teleport
	fastEventually(t, func() bool {
		tUser, err := tClient.GetUser(userName, false)
		require.NoError(t, err)

		// TeleportUser was updated with new roles
		return assert.ElementsMatch(t, tUser.GetRoles(), []string{"x", "z"})
	})

	// Updating the user in K8S
	var k8sUserNewVersion resourcesv2.TeleportUser
	err = k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
		Name:      userName,
	}, &k8sUserNewVersion)
	require.NoError(t, err)

	k8sUserNewVersion.Spec.Roles = append(k8sUserNewVersion.Spec.Roles, "y")
	err = k8sClient.Update(ctx, &k8sUserNewVersion)
	require.NoError(t, err)

	// Updates the user in Teleport
	fastEventually(t, func() bool {
		tUser, err := tClient.GetUser(userName, false)
		require.NoError(t, err)

		// TeleportUser updated with new roles
		return assert.ElementsMatch(t, tUser.GetRoles(), []string{"x", "z", "y"})
	})
}

func k8sCreateDummyUser(ctx context.Context, t *testing.T, kc kclient.Client, namespace, userName string) {
	user := resourcesv2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
		Spec: resourcesv2.TeleportUserSpec{
			Roles: []string{"a", "b"},
		},
	}
	k8sCreateUser(ctx, t, kc, &user)
}

func k8sDeleteUser(ctx context.Context, t *testing.T, kc kclient.Client, userName, namespace string) {
	user := resourcesv2.TeleportUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
	}
	err := kc.Delete(ctx, &user)
	require.NoError(t, err)
}

func k8sCreateUser(ctx context.Context, t *testing.T, kc kclient.Client, user *resourcesv2.TeleportUser) {
	err := kc.Create(ctx, user)
	require.NoError(t, err)
}

func TestAddTeleportResourceOriginUser(t *testing.T) {
	r := UserReconciler{}
	tests := []struct {
		name     string
		resource types.User
	}{
		{
			name: "origin already set correctly",
			resource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
		},
		{
			name: "origin already set incorrectly",
			resource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
		},
		{
			name: "origin not set",
			resource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "user with correct origin",
					Labels: map[string]string{"foo": "bar"},
				},
			},
		},
		{
			name: "no labels",
			resource: &types.UserV2{
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
