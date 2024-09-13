// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubewaitingcontainerv1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestKubeWaitingContainerServiceCRUD(t *testing.T) {
	t.Parallel()

	const username = "user"
	const cluster = "cluster"
	const namespace = "default"
	const podName = "pod"
	const patchType = kubewaitingcontainer.JSONPatchType

	sampleKubeWaitingContFn := func(t *testing.T, name string) *kubewaitingcontainerpb.KubernetesWaitingContainer {
		wc, err := kubewaitingcontainer.NewKubeWaitingContainer(
			name,
			&kubewaitingcontainerpb.KubernetesWaitingContainerSpec{
				Username:      username,
				Cluster:       cluster,
				Namespace:     namespace,
				PodName:       podName,
				ContainerName: name,
				Patch:         []byte("patch"),
				PatchType:     patchType,
			},
		)
		require.NoError(t, err)
		return wc
	}

	kubeAuthFn := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		authzContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
			Role:     types.RoleKube,
			Username: string(types.RoleKube),
		}, &types.SessionRecordingConfigV2{})
		require.NoError(t, err)
		return authzContext, nil
	})

	tt := []struct {
		Name       string
		Authorizer func(t *testing.T, client localClient) authz.Authorizer
		Setup      func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string)
		Test       func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string)
	}{
		// List
		{
			Name: "allowed list access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Setup: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				for i := 0; i < 10; i++ {
					_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
						WaitingContainer: sampleKubeWaitingContFn(t, uuid.NewString()),
					})
					require.NoError(t, err)
				}
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.ListKubernetesWaitingContainers(ctx, &kubewaitingcontainerpb.ListKubernetesWaitingContainersRequest{})
				require.NoError(t, err)
			},
		},
		{
			Name: "not allowed list access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return authorizerForDummyUser(t, ctx, client, []string{types.VerbRead, types.VerbList}), nil
				})
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.ListKubernetesWaitingContainers(ctx, &kubewaitingcontainerpb.ListKubernetesWaitingContainersRequest{})
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},

		// Get
		{
			Name: "allowed get access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Setup: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.NoError(t, err)
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				out, err := resourceSvc.GetKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.NoError(t, err)
				require.Equal(t, wcName, out.Metadata.Name)
				require.Equal(t, username, out.Spec.Username)
				require.Equal(t, cluster, out.Spec.Cluster)
				require.Equal(t, namespace, out.Spec.Namespace)
				require.Equal(t, podName, out.Spec.PodName)
				require.Equal(t, wcName, out.Spec.ContainerName)
				require.Equal(t, patchType, out.Spec.PatchType)
			},
		},
		{
			Name: "not allowed get access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return authorizerForDummyUser(t, ctx, client, []string{types.VerbRead, types.VerbList}), nil
				})
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.GetKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			Name: "get nonexistent resource",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.GetKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},

		// Create
		{
			Name: "allowed create access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				out, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.NoError(t, err)
				require.Equal(t, wcName, out.Metadata.Name)
				require.Equal(t, username, out.Spec.Username)
				require.Equal(t, cluster, out.Spec.Cluster)
				require.Equal(t, namespace, out.Spec.Namespace)
				require.Equal(t, podName, out.Spec.PodName)
				require.Equal(t, wcName, out.Spec.ContainerName)
				require.Equal(t, patchType, out.Spec.PatchType)
			},
		},
		{
			Name: "not allowed create access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return authorizerForDummyUser(t, ctx, client, []string{types.VerbRead, types.VerbList}), nil
				})
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			Name: "create resource twice",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Setup: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.NoError(t, err)
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.Error(t, err)
				require.True(t, trace.IsAlreadyExists(err))
			},
		},

		// Delete
		{
			Name: "allowed delete access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Setup: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.CreateKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.CreateKubernetesWaitingContainerRequest{
					WaitingContainer: sampleKubeWaitingContFn(t, wcName),
				})
				require.NoError(t, err)
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.NoError(t, err)
			},
		},
		{
			Name: "not allowed delete access",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return authorizerForDummyUser(t, ctx, client, []string{types.VerbRead, types.VerbList}), nil
				})
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			Name: "get nonexistent resource",
			Authorizer: func(t *testing.T, client localClient) authz.Authorizer {
				return kubeAuthFn
			},
			Test: func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string) {
				_, err := resourceSvc.DeleteKubernetesWaitingContainer(ctx, &kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest{
					Username:      username,
					Cluster:       cluster,
					Namespace:     namespace,
					PodName:       podName,
					ContainerName: wcName,
				})
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx, _, resourceSvc := initSvc(t, tc.Authorizer)

			wcName := uuid.NewString()
			if tc.Setup != nil {
				tc.Setup(t, ctx, resourceSvc, wcName)
			}

			tc.Test(t, ctx, resourceSvc, wcName)
		})
	}
}

func authorizerForDummyUser(t *testing.T, ctx context.Context, localClient localClient, roleVerbs []string) *authz.Context {
	const clusterName = "localhost"

	// Create role
	roleName := "role-" + uuid.NewString()
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{{
			Resources: []string{types.KindKubeWaitingContainer},
			Verbs:     roleVerbs,
		}}},
	})
	require.NoError(t, err)

	err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	err = localClient.CreateUser(user)
	require.NoError(t, err)

	localUser := authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	}
	authCtx, err := authz.ContextForLocalUser(localUser, localClient, clusterName, true)
	require.NoError(t, err)

	return authCtx
}

type localClient interface {
	authz.AuthorizerAccessPoint

	CreateRole(ctx context.Context, role types.Role) error
	CreateUser(user types.User) error
}

func initSvc(t *testing.T, authorizerFn func(t *testing.T, client localClient) authz.Authorizer) (context.Context, localClient, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)
	clusterSrv, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	caSrv := local.NewCAService(backend)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	err = clusterConfigSvc.SetAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	err = clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig())
	require.NoError(t, err)
	err = clusterConfigSvc.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	err = clusterConfigSvc.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	localResourceService, err := local.NewKubeWaitingContainerService(backend)
	require.NoError(t, err)

	client := struct {
		*local.AccessService
		*local.IdentityService
		*local.ClusterConfigurationService
		*local.CA
	}{
		AccessService:               roleSvc,
		IdentityService:             userSvc,
		ClusterConfigurationService: clusterSrv,
		CA:                          caSrv,
	}

	resourceSvc, err := NewService(ServiceConfig{
		Authorizer: authorizerFn(t, client),
		Backend:    localResourceService,
		Cache:      localResourceService,
	})
	require.NoError(t, err)

	return ctx, client, resourceSvc
}
