/*
Copyright 2023 Gravitational, Inc.

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

package discoveryconfigv1

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	convert "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestDiscoveryConfigCRUD(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"

	requireTraceErrorFn := func(traceFn func(error) bool) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...interface{}) {
			require.True(t, traceFn(err), "received an un-expected error: %v", err)
		}
	}

	ctx, localClient, resourceSvc := initSvc(t, clusterName)

	sampleDiscoveryConfigFn := func(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
		dc, err := discoveryconfig.NewDiscoveryConfig(
			header.Metadata{Name: name},
			discoveryconfig.Spec{
				DiscoveryGroup: "some-group",
			},
		)
		require.NoError(t, err)
		return dc
	}

	tt := []struct {
		Name         string
		Role         types.RoleSpecV6
		Setup        func(t *testing.T, dcName string)
		Test         func(ctx context.Context, resourceSvc *Service, dcName string) error
		ErrAssertion require.ErrorAssertionFunc
	}{
		// Read
		{
			Name: "allowed read access to discovery configs",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, dcName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.GetDiscoveryConfig(ctx, &discoveryconfigpb.GetDiscoveryConfigRequest{
					Name: dcName,
				})
				return err
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no access to read discovery configs",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.GetDiscoveryConfig(ctx, &discoveryconfigpb.GetDiscoveryConfigRequest{
					Name: dcName,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "denied access to read discovery configs",
			Role: types.RoleSpecV6{
				Deny: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbRead},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.GetDiscoveryConfig(ctx, &discoveryconfigpb.GetDiscoveryConfigRequest{
					Name: dcName,
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},

		// List
		{
			Name: "allowed list access to discovery configs",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbList, types.VerbRead},
				}}},
			},
			Setup: func(t *testing.T, _ string) {
				for i := 0; i < 10; i++ {
					_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, uuid.NewString()))
					require.NoError(t, err)
				}
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.ListDiscoveryConfigs(ctx, &discoveryconfigpb.ListDiscoveryConfigsRequest{
					PageSize:  0,
					NextToken: "",
				})
				return err
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no list access to discovery config",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.ListDiscoveryConfigs(ctx, &discoveryconfigpb.ListDiscoveryConfigsRequest{
					PageSize:  0,
					NextToken: "",
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},

		// Create
		{
			Name: "no access to create discovery configs",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.CreateDiscoveryConfig(ctx, &discoveryconfigpb.CreateDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to create discovery configs",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbCreate},
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.CreateDiscoveryConfig(ctx, &discoveryconfigpb.CreateDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Update
		{
			Name: "no access to update discovery config",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.UpdateDiscoveryConfig(ctx, &discoveryconfigpb.UpdateDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to update discovery config",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbUpdate},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, dcName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.UpdateDiscoveryConfig(ctx, &discoveryconfigpb.UpdateDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Upsert
		{
			Name: "no access to upsert discovery config",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbUpdate}, // missing VerbCreate
				}}},
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.UpsertDiscoveryConfig(ctx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to upsert discovery config",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbUpdate, types.VerbCreate},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				dc := sampleDiscoveryConfigFn(t, dcName)
				_, err := resourceSvc.UpsertDiscoveryConfig(ctx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{
					DiscoveryConfig: convert.ToProto(dc),
				})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Delete
		{
			Name: "no access to delete discovery config",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.DeleteDiscoveryConfig(ctx, &discoveryconfigpb.DeleteDiscoveryConfigRequest{Name: "x"})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "access to delete discovery config",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, dcName))
				require.NoError(t, err)
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.DeleteDiscoveryConfig(ctx, &discoveryconfigpb.DeleteDiscoveryConfigRequest{Name: dcName})
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Delete all
		{
			Name: "remove all discovery configs fails when no access",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.DeleteAllDiscoveryConfigs(ctx, &discoveryconfigpb.DeleteAllDiscoveryConfigsRequest{})
				return err
			},
			ErrAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			Name: "remove all discovery configs",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbDelete},
				}}},
			},
			Setup: func(t *testing.T, _ string) {
				for i := 0; i < 10; i++ {
					_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, uuid.NewString()))
					require.NoError(t, err)
				}
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.DeleteAllDiscoveryConfigs(ctx, &discoveryconfigpb.DeleteAllDiscoveryConfigsRequest{})
				return err
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
		})
	}
}

func TestUpdateDiscoveryConfigStatus(t *testing.T) {
	clusterName := "test-cluster"

	requireTraceErrorFn := func(traceFn func(error) bool) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...interface{}) {
			require.True(t, traceFn(err), "received an un-expected error: %v", err)
		}
	}

	ctx, localClient, resourceSvc := initSvc(t, clusterName)

	sampleDiscoveryConfigFn := func(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
		dc, err := discoveryconfig.NewDiscoveryConfig(
			header.Metadata{Name: name},
			discoveryconfig.Spec{
				DiscoveryGroup: "some-group",
			},
		)
		require.NoError(t, err)
		return dc
	}

	tt := []struct {
		name         string
		systemRole   types.SystemRole
		setup        func(t *testing.T, dcName string)
		test         func(t *testing.T, ctx context.Context, resourceSvc *Service, dcName string) error
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:       "no access to update discovery config status",
			systemRole: types.RoleNode,
			test: func(t *testing.T, ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{
					Name: dcName,
				})
				return err
			},
			errAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			name:       "discovery config doesn't exist",
			systemRole: types.RoleDiscovery,
			test: func(t *testing.T, ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{
					Name: dcName,
				})
				return err
			},
			errAssertion: requireTraceErrorFn(trace.IsNotFound),
		},
		{
			name:       "access to update discovery config status",
			systemRole: types.RoleDiscovery,
			setup: func(t *testing.T, dcName string) {
				_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, dcName))
				require.NoError(t, err)
			},
			test: func(t *testing.T, ctx context.Context, resourceSvc *Service, dcName string) error {
				now := time.Now()
				msg := "error message"
				status := &discoveryconfigpb.DiscoveryConfigStatus{
					State:               discoveryconfigpb.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING,
					ErrorMessage:        &msg,
					DiscoveredResources: 42,
					LastSyncTime:        timestamppb.New(now),
				}

				out, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{
					Name:   dcName,
					Status: status,
				})
				require.NoError(t, err)
				dc := sampleDiscoveryConfigFn(t, dcName)
				dc.Status = convert.StatusFromProto(status)

				outL, err := convert.FromProto(out)
				require.NoError(t, err)
				// copy revision from the output
				dc.Metadata.Revision = outL.Metadata.Revision
				require.Equal(t, dc, outL)
				return nil
			},
			errAssertion: require.NoError,
		},
	}
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			localCtx := authorizerForSystemRole(ctx, string(tc.systemRole))

			dcName := uuid.NewString()
			if tc.setup != nil {
				tc.setup(t, dcName)
			}

			err := tc.test(t, localCtx, resourceSvc, dcName)
			tc.errAssertion(t, err)
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

func authorizerForSystemRole(ctx context.Context, systemRole string) context.Context {
	return authz.ContextWithUser(ctx, authz.BuiltinRole{
		Username: uuid.NewString(),
		Role:     types.SystemRole(systemRole),
		Identity: tlsca.Identity{
			SystemRoles: []string{systemRole},
			Groups:      []string{systemRole},
		},
	})
}

type localClient interface {
	CreateUser(user types.User) error
	CreateRole(ctx context.Context, role types.Role) error
	CreateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
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

	localResourceService, err := local.NewDiscoveryConfigService(backend)
	require.NoError(t, err)

	emitter := events.NewDiscardEmitter()

	resourceSvc, err := NewService(ServiceConfig{
		Backend:    localResourceService,
		Authorizer: authorizer,
		Emitter:    emitter,
	})
	require.NoError(t, err)

	return ctx, struct {
		*local.AccessService
		*local.IdentityService
		*local.DiscoveryConfigService
	}{
		AccessService:          roleSvc,
		IdentityService:        userSvc,
		DiscoveryConfigService: localResourceService,
	}, resourceSvc
}
