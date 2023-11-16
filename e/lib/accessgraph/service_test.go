/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessgraph

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestService_Query(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	agSrv, roleSvc, userSvc, testFake := setup(ctx, t)

	createUsersAndRoles(t, roleSvc, ctx, userSvc)

	t.Run("Query is not allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{}, map[string][]string{})

		_, err := agSrv.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.ErrorContains(t, err, "not allowed to read the access graph")

		require.Equal(t, 0, testFake.calledFunctions["Query"])
	})

	t.Run("Query is allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user", []string{"test-role"}, map[string][]string{})

		_, err := agSrv.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.NoError(t, err)

		require.Equal(t, 1, testFake.calledFunctions["Query"])
	})

	t.Run("GetFile is always allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{}, map[string][]string{})

		_, err := agSrv.GetFile(ctx, &accessgraphv1alpha.GetFileRequest{})
		require.NoError(t, err)
	})
}

type serviceFake struct {
	calledFunctions map[string]int // map of function name to number of times called
}

func newServiceFake() *serviceFake {
	return &serviceFake{
		calledFunctions: make(map[string]int),
	}
}

func (s *serviceFake) Query(_ context.Context, _ *accessgraphv1alpha.QueryRequest, _ ...grpc.CallOption) (*accessgraphv1alpha.QueryResponse, error) {
	s.calledFunctions["Query"]++
	return &accessgraphv1alpha.QueryResponse{}, nil
}

func (s *serviceFake) GetFile(_ context.Context, _ *accessgraphv1alpha.GetFileRequest, _ ...grpc.CallOption) (*accessgraphv1alpha.GetFileResponse, error) {
	s.calledFunctions["GetFile"]++
	return &accessgraphv1alpha.GetFileResponse{}, nil
}

func (s *serviceFake) EventsStream(_ context.Context, _ ...grpc.CallOption) (accessgraphv1alpha.AccessGraphService_EventsStreamClient, error) {
	s.calledFunctions["EventsStream"]++
	return nil, nil
}

func setup(ctx context.Context, t *testing.T) (*Service, *local.AccessService, *local.IdentityService, *serviceFake) {
	t.Helper()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)

	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewIdentityService(backend)

	require.NoError(t, clusterConfigSvc.SetAuthPreference(ctx, types.DefaultAuthPreference()))
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	require.NoError(t, clusterConfigSvc.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, clusterConfigSvc.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))

	accessPoint := struct {
		services.ClusterConfiguration
		services.Trust
		services.RoleGetter
		services.UserGetter
	}{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	testFake := newServiceFake()
	serviceNew, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Logger:     utils.NewLoggerForTests(),
		Client:     testFake,
	})
	require.NoError(t, err)

	return serviceNew, roleSvc, userSvc, testFake
}

func createUsersAndRoles(t *testing.T, roleSvc *local.AccessService, ctx context.Context, userSvc *local.IdentityService) {
	t.Helper()

	testRole, err := types.NewRole("test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces: []string{"*"},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAccessGraph},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})

	require.NoError(t, err)

	_, err = roleSvc.CreateRole(ctx, testRole)
	require.NoError(t, err)

	testUser, err := types.NewUser("test-user")
	require.NoError(t, err)

	testUser.SetRoles([]string{})
	_, err = userSvc.CreateUser(ctx, testUser)
	require.NoError(t, err)

	_, err = types.NewUser("test-user-no-perm")
	require.NoError(t, err)
}

func genUserContext(ctx context.Context, username string, groups []string, traits map[string][]string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
			Groups:   groups,
			Traits:   traits,
		},
	})
}
