/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package kubewaitingcontainer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services/local"
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
		Authorizer func(t *testing.T, client test.LocalClient) authz.Authorizer
		Setup      func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string)
		Test       func(t *testing.T, ctx context.Context, resourceSvc *Service, wcName string)
	}{
		// List
		{
			Name: "allowed list access",
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return test.AuthorizerForDummyUser(t, ctx, client, types.KindKubeWaitingContainer, []string{types.VerbRead, types.VerbList}), nil
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return test.AuthorizerForDummyUser(t, ctx, client, types.KindKubeWaitingContainer, []string{types.VerbRead, types.VerbList}), nil
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return test.AuthorizerForDummyUser(t, ctx, client, types.KindKubeWaitingContainer, []string{types.VerbRead, types.VerbList}), nil
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
				return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return test.AuthorizerForDummyUser(t, ctx, client, types.KindKubeWaitingContainer, []string{types.VerbRead, types.VerbList}), nil
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
			Name: "delete nonexistent resource",
			Authorizer: func(t *testing.T, client test.LocalClient) authz.Authorizer {
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

			ctx, resourceSvc := initSvc(t, tc.Authorizer)

			wcName := uuid.NewString()
			if tc.Setup != nil {
				tc.Setup(t, ctx, resourceSvc, wcName)
			}

			tc.Test(t, ctx, resourceSvc, wcName)
		})
	}
}

func initSvc(t *testing.T, authorizerFn func(t *testing.T, client test.LocalClient) authz.Authorizer) (context.Context, *Service) {
	ctx, client, backend := test.InitRBACServices(t)
	localResourceService, err := local.NewKubeWaitingContainerService(backend)
	require.NoError(t, err)

	resourceSvc, err := NewService(ServiceConfig{
		Authorizer: authorizerFn(t, client),
		Backend:    localResourceService,
		Cache:      localResourceService,
	})
	require.NoError(t, err)
	return ctx, resourceSvc
}
