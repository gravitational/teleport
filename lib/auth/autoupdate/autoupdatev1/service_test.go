// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package autoupdatev1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAutoUpdateCRUD(t *testing.T) {
	t.Parallel()

	requireTraceErrorFn := func(traceFn func(error) bool) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...interface{}) {
			require.True(t, traceFn(err), "received an un-expected error: %v", err)
		}
	}

	ctx, localClient, resourceSvc := initSvc(t, "test-cluster")
	initialConfig, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		ToolsAutoupdate: true,
	})
	require.NoError(t, err)
	initialVersion, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		ToolsVersion: "1.2.3",
	})
	require.NoError(t, err)

	tt := []struct {
		Name         string
		Role         types.RoleSpecV6
		Setup        func(t *testing.T, dcName string)
		Test         func(ctx context.Context, resourceSvc *Service, dcName string) error
		ErrAssertion require.ErrorAssertionFunc
	}{
		// Read
		{
			Name: "allowed read access to auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateAutoUpdateConfig(ctx, initialConfig)
				require.NoError(t, err)
				_, err = localClient.CreateAutoUpdateVersion(ctx, initialVersion)
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.GetAutoUpdateConfig(ctx, &autoupdatev1pb.GetAutoUpdateConfigRequest{})
				_, errVersion := resourceSvc.GetAutoUpdateVersion(ctx, &autoupdatev1pb.GetAutoUpdateVersionRequest{})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no access to auto update resources",
			Role: types.RoleSpecV6{},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateAutoUpdateConfig(ctx, initialConfig)
				require.NoError(t, err)
				_, err = localClient.CreateAutoUpdateVersion(ctx, initialVersion)
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.GetAutoUpdateConfig(ctx, &autoupdatev1pb.GetAutoUpdateConfigRequest{})
				_, errVersion := resourceSvc.GetAutoUpdateVersion(ctx, &autoupdatev1pb.GetAutoUpdateVersionRequest{})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		// Create
		{
			Name: "no access to create auto update resources",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.CreateAutoUpdateConfig(ctx, &autoupdatev1pb.CreateAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.CreateAutoUpdateVersion(ctx, &autoupdatev1pb.CreateAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to create auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.CreateAutoUpdateConfig(ctx, &autoupdatev1pb.CreateAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.CreateAutoUpdateVersion(ctx, &autoupdatev1pb.CreateAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: require.NoError,
		},
		// Update
		{
			Name: "no access to update auto update resources",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.UpdateAutoUpdateConfig(ctx, &autoupdatev1pb.UpdateAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.UpdateAutoUpdateVersion(ctx, &autoupdatev1pb.UpdateAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to update auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbUpdate},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateAutoUpdateConfig(ctx, initialConfig)
				require.NoError(t, err)
				_, err = localClient.CreateAutoUpdateVersion(ctx, initialVersion)
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.UpdateAutoUpdateConfig(ctx, &autoupdatev1pb.UpdateAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.UpdateAutoUpdateVersion(ctx, &autoupdatev1pb.UpdateAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: require.NoError,
		},
		// Upsert
		{
			Name: "no access to upsert auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbUpdate}, // missing VerbCreate
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.UpsertAutoUpdateConfig(ctx, &autoupdatev1pb.UpsertAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.UpsertAutoUpdateVersion(ctx, &autoupdatev1pb.UpsertAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to upsert auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.UpsertAutoUpdateConfig(ctx, &autoupdatev1pb.UpsertAutoUpdateConfigRequest{
					Config: initialConfig,
				})
				_, errVersion := resourceSvc.UpsertAutoUpdateVersion(ctx, &autoupdatev1pb.UpsertAutoUpdateVersionRequest{
					Version: initialVersion,
				})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: require.NoError,
		},
		// Delete
		{
			Name: "no access to delete auto update resources",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.DeleteAutoUpdateConfig(ctx, &autoupdatev1pb.DeleteAutoUpdateConfigRequest{})
				_, errVersion := resourceSvc.DeleteAutoUpdateVersion(ctx, &autoupdatev1pb.DeleteAutoUpdateVersionRequest{})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to delete auto update resources",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindAutoUpdateConfig, types.KindAutoUpdateVersion},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateAutoUpdateConfig(ctx, initialConfig)
				require.NoError(t, err)
				_, err = localClient.CreateAutoUpdateVersion(ctx, initialVersion)
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, errConfig := resourceSvc.DeleteAutoUpdateConfig(ctx, &autoupdatev1pb.DeleteAutoUpdateConfigRequest{})
				_, errVersion := resourceSvc.DeleteAutoUpdateVersion(ctx, &autoupdatev1pb.DeleteAutoUpdateVersionRequest{})
				return trace.NewAggregate(errConfig, errVersion)
			},
			ErrAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			localCtx := authorizerForDummyUser(t, ctx, tc.Role, localClient)

			dcName := uuid.NewString()
			if tc.Setup != nil {
				tc.Setup(t, dcName)
			}

			err := tc.Test(localCtx, resourceSvc, dcName)
			tc.ErrAssertion(t, err)
			err = localClient.DeleteAutoUpdateConfig(ctx)
			if !trace.IsNotFound(err) {
				require.NoError(t, err)
			}
			err = localClient.DeleteAutoUpdateVersion(ctx)
			if !trace.IsNotFound(err) {
				require.NoError(t, err)
			}
		})
	}
}

func authorizerForDummyUser(t *testing.T, ctx context.Context, roleSpec types.RoleSpecV6, localClient localClient) context.Context {
	// Create role
	roleName := "role-" + uuid.NewString()
	role, err := types.NewRole(roleName, roleSpec)
	require.NoError(t, err)

	err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	err = localClient.CreateUser(user)
	require.NoError(t, err)

	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username: user.GetName(),
			Groups:   []string{role.GetName()},
		},
	})
}

type localClient interface {
	CreateUser(user types.User) error
	CreateRole(ctx context.Context, role types.Role) error
	services.AutoUpdateService
}

func initSvc(t *testing.T, clusterName string) (context.Context, localClient, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
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
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	localResourceService, err := local.NewAutoUpdateService(backend)
	require.NoError(t, err)

	resourceSvc, err := NewService(ServiceConfig{
		Backend:    localResourceService,
		Cache:      localResourceService,
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	return ctx, struct {
		*local.AccessService
		*local.IdentityService
		*local.AutoUpdateService
	}{
		AccessService:     roleSvc,
		IdentityService:   userSvc,
		AutoUpdateService: localResourceService,
	}, resourceSvc
}
