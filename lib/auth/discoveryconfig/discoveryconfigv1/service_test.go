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

package discoveryconfigv1

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/metadata"
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
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestDiscoveryConfigCRUD(t *testing.T) {
	t.Parallel()
	clusterName := "test-cluster"

	requireTraceErrorFn := func(traceFn func(error) bool) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...any) {
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
				_, err := resourceSvc.GetDiscoveryConfig(ctx, discoveryconfigpb.GetDiscoveryConfigRequest_builder{
					Name: dcName,
				}.Build())
				return err
			},
			ErrAssertion: require.NoError,
		},
		{
			Name: "no access to read discovery configs",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.GetDiscoveryConfig(ctx, discoveryconfigpb.GetDiscoveryConfigRequest_builder{
					Name: dcName,
				}.Build())
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
				_, err := resourceSvc.GetDiscoveryConfig(ctx, discoveryconfigpb.GetDiscoveryConfigRequest_builder{
					Name: dcName,
				}.Build())
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
				for range 10 {
					_, err := localClient.CreateDiscoveryConfig(ctx, sampleDiscoveryConfigFn(t, uuid.NewString()))
					require.NoError(t, err)
				}
			},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.ListDiscoveryConfigs(ctx, discoveryconfigpb.ListDiscoveryConfigsRequest_builder{
					PageSize:  0,
					NextToken: "",
				}.Build())
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
				_, err := resourceSvc.ListDiscoveryConfigs(ctx, discoveryconfigpb.ListDiscoveryConfigsRequest_builder{
					PageSize:  0,
					NextToken: "",
				}.Build())
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
				_, err := resourceSvc.CreateDiscoveryConfig(ctx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
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
				_, err := resourceSvc.CreateDiscoveryConfig(ctx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
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
				_, err := resourceSvc.UpdateDiscoveryConfig(ctx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
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
				_, err := resourceSvc.UpdateDiscoveryConfig(ctx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
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
				_, err := resourceSvc.UpsertDiscoveryConfig(ctx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
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
				_, err := resourceSvc.UpsertDiscoveryConfig(ctx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{
					DiscoveryConfig: convert.ToProto(dc),
				}.Build())
				return err
			},
			ErrAssertion: require.NoError,
		},

		// Delete
		{
			Name: "no access to delete discovery config",
			Role: types.RoleSpecV6{},
			Test: func(ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.DeleteDiscoveryConfig(ctx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: "x"}.Build())
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
				_, err := resourceSvc.DeleteDiscoveryConfig(ctx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: dcName}.Build())
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
				for range 10 {
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
		return func(tt require.TestingT, err error, i ...any) {
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
				_, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{
					Name: dcName,
				}.Build())
				return err
			},
			errAssertion: requireTraceErrorFn(trace.IsAccessDenied),
		},
		{
			name:       "discovery config doesn't exist",
			systemRole: types.RoleDiscovery,
			test: func(t *testing.T, ctx context.Context, resourceSvc *Service, dcName string) error {
				_, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{
					Name: dcName,
				}.Build())
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
				status := discoveryconfigpb.DiscoveryConfigStatus_builder{
					State:               discoveryconfigpb.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING,
					ErrorMessage:        &msg,
					DiscoveredResources: 42,
					LastSyncTime:        timestamppb.New(now),
				}.Build()

				out, err := resourceSvc.UpdateDiscoveryConfigStatus(ctx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{
					Name:   dcName,
					Status: status,
				}.Build())
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

	role, err = localClient.CreateRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user, err := types.NewUser("user-" + uuid.NewString())
	require.NoError(t, err)
	user.AddRole(roleName)
	user, err = localClient.CreateUser(ctx, user)
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
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	CreateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
	services.Presence
}

type initSvcOption func(*ServiceConfig)

func withTeleportCloud() initSvcOption {
	return func(cfg *ServiceConfig) { cfg.IsTeleportCloud = true }
}

func initSvc(t *testing.T, clusterName string, opts ...initSvcOption) (context.Context, localClient, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc, err := local.NewTestIdentityService(backend)
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
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

	cfg := ServiceConfig{
		Backend:       localResourceService,
		Authorizer:    authorizer,
		Emitter:       events.NewDiscardEmitter(),
		UsageReporter: usagereporter.DiscardUsageReporter{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	resourceSvc, err := NewService(cfg)
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

func TestExtractDiscoveryConfigMetadata(t *testing.T) {
	t.Parallel()

	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "test"},
		discoveryconfig.Spec{
			DiscoveryGroup: "group",
			AWS: []types.AWSMatcher{
				{Types: []string{"ec2", "rds"}, Regions: []string{"us-east-1"}},
				{Types: []string{"ec2"}, Regions: []string{"us-east-1"}}, // duplicate should be deduped
			},
			Azure: []types.AzureMatcher{
				{Types: []string{"aks"}},
			},
			GCP: []types.GCPMatcher{
				{Types: []string{"gke"}, ProjectIDs: []string{"my-project"}},
			},
			Kube: []types.KubernetesMatcher{
				{Types: []string{"app"}},
			},
		},
	)
	require.NoError(t, err)

	resourceTypes, cloudProviders := extractDiscoveryConfigMetadata(dc)

	require.ElementsMatch(t, []string{"aws:ec2", "aws:rds", "azure:aks", "gcp:gke", "k8s:app"}, resourceTypes)
	require.ElementsMatch(t, []string{"aws", "azure", "gcp", "k8s"}, cloudProviders)
}

func TestDowngrade(t *testing.T) {
	for _, tc := range []struct {
		name          string
		clientVersion string
		input         *discoveryconfig.DiscoveryConfig
		expected      *discoveryconfig.DiscoveryConfig
	}{
		{
			name:          "no downgrade for recent client",
			clientVersion: "18.5.0",
			input: func() *discoveryconfig.DiscoveryConfig {
				dc, err := discoveryconfig.NewDiscoveryConfig(
					header.Metadata{Name: "dc1"},
					discoveryconfig.Spec{
						DiscoveryGroup: "group1",
						AWS: []types.AWSMatcher{
							{
								Regions: []string{types.Wildcard},
								Types:   []string{"ec2"},
							},
						},
					},
				)
				require.NoError(t, err)
				return dc
			}(),
			expected: func() *discoveryconfig.DiscoveryConfig {
				dc, err := discoveryconfig.NewDiscoveryConfig(
					header.Metadata{Name: "dc1"},
					discoveryconfig.Spec{
						DiscoveryGroup: "group1",
						AWS: []types.AWSMatcher{
							{
								Regions: []string{types.Wildcard},
								Types:   []string{"ec2"},
							},
						},
					},
				)
				require.NoError(t, err)
				return dc
			}(),
		},
		{
			name:          "downgrade for old client",
			clientVersion: "18.4.2",
			input: func() *discoveryconfig.DiscoveryConfig {
				dc, err := discoveryconfig.NewDiscoveryConfig(
					header.Metadata{Name: "dc1"},
					discoveryconfig.Spec{
						DiscoveryGroup: "group1",
						AWS: []types.AWSMatcher{
							{
								Regions: []string{types.Wildcard},
								Types:   []string{"ec2"},
							},
						},
					},
				)
				require.NoError(t, err)
				return dc
			}(),
			expected: func() *discoveryconfig.DiscoveryConfig {
				dc, err := discoveryconfig.NewDiscoveryConfig(
					header.Metadata{Name: "dc1"},
					discoveryconfig.Spec{
						DiscoveryGroup: "group1",
						AWS: []types.AWSMatcher{
							{
								Regions: []string{types.Wildcard},
								Types:   []string{"ec2"},
							},
						},
					},
				)
				require.NoError(t, err)
				return dc
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.AddMetadataToContext(t.Context(), map[string]string{
				metadata.VersionKey: tc.clientVersion,
			})
			downgraded, err := MaybeDowngradeDiscoveryConfig(ctx, tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, downgraded)
		})
	}
}

func TestValidateDiscoveryConfigCalledInMutateMethods(t *testing.T) {
	t.Parallel()

	ctx, localClient, svc := initSvc(t, "test-cluster", withTeleportCloud())
	ctx = authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{{
			Resources: []string{types.KindDiscoveryConfig},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		}}},
	}, localClient)

	invalidDC, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "cloud-dc"},
		discoveryconfig.Spec{
			DiscoveryGroup: "cloud-discovery-group",
			AWS:            []types.AWSMatcher{{Types: []string{"ec2"}, Regions: []string{"us-east-1"}}},
		},
	)
	require.NoError(t, err)
	invalidProto := convert.ToProto(invalidDC)

	const wantMsg = `AWS matchers in discovery configs targeting the "cloud-discovery-group" discovery group must specify an integration`

	_, err = svc.CreateDiscoveryConfig(ctx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: invalidProto}.Build())
	require.True(t, trace.IsBadParameter(err), "Create: expected BadParameter, got: %v", err)
	require.ErrorContains(t, err, wantMsg)

	_, err = svc.UpdateDiscoveryConfig(ctx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: invalidProto}.Build())
	require.True(t, trace.IsBadParameter(err), "Update: expected BadParameter, got: %v", err)
	require.ErrorContains(t, err, wantMsg)

	_, err = svc.UpsertDiscoveryConfig(ctx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: invalidProto}.Build())
	require.True(t, trace.IsBadParameter(err), "Upsert: expected BadParameter, got: %v", err)
	require.ErrorContains(t, err, wantMsg)
}

func TestValidateDiscoveryConfigIntegrationFields(t *testing.T) {
	t.Parallel()

	svc := &Service{isTeleportCloud: true}
	const cloudGroup = "cloud-discovery-group"

	makeDC := func(t *testing.T, spec discoveryconfig.Spec) *discoveryconfig.DiscoveryConfig {
		t.Helper()
		spec.DiscoveryGroup = cloudGroup
		dc, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "test"}, spec)
		require.NoError(t, err)
		return dc
	}

	tests := []struct {
		name    string
		dc      *discoveryconfig.DiscoveryConfig
		wantErr bool
	}{
		{
			name: "AWS matcher without integration is rejected",
			dc: makeDC(t, discoveryconfig.Spec{
				AWS: []types.AWSMatcher{{Types: []string{"ec2"}, Regions: []string{"us-east-1"}}},
			}),
			wantErr: true,
		},
		{
			name: "AWS matcher with integration is accepted",
			dc: makeDC(t, discoveryconfig.Spec{
				// Use "rds" to avoid triggering EC2 Instance Connect Endpoint validation.
				AWS: []types.AWSMatcher{{Types: []string{"rds"}, Regions: []string{"us-east-1"}, Integration: "my-integration"}},
			}),
			wantErr: false,
		},
		{
			name: "Azure matcher without integration is rejected",
			dc: makeDC(t, discoveryconfig.Spec{
				Azure: []types.AzureMatcher{{Types: []string{"aks"}}},
			}),
			wantErr: true,
		},
		{
			name: "Azure matcher with integration is accepted",
			dc: makeDC(t, discoveryconfig.Spec{
				Azure: []types.AzureMatcher{{Types: []string{"aks"}, Integration: "my-integration"}},
			}),
			wantErr: false,
		},
		{
			name: "AccessGraph AWS sync without integration is rejected",
			dc: makeDC(t, discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
				},
			}),
			wantErr: true,
		},
		{
			name: "AccessGraph AWS sync with integration is accepted",
			dc: makeDC(t, discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}, Integration: "my-integration"}},
				},
			}),
			wantErr: false,
		},
		{
			name: "AccessGraph Azure sync without integration is rejected",
			dc: makeDC(t, discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub-1"}},
				},
			}),
			wantErr: true,
		},
		{
			name: "AccessGraph Azure sync with integration is accepted",
			dc: makeDC(t, discoveryconfig.Spec{
				AccessGraph: &types.AccessGraphSync{
					Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub-1", Integration: "my-integration"}},
				},
			}),
			wantErr: false,
		},
		{
			name: "non-cloud-discovery group skips integration validation",
			dc: func() *discoveryconfig.DiscoveryConfig {
				dc, err := discoveryconfig.NewDiscoveryConfig(
					header.Metadata{Name: "test"},
					discoveryconfig.Spec{
						DiscoveryGroup: "other-group",
						AWS:            []types.AWSMatcher{{Types: []string{"ec2"}, Regions: []string{"us-east-1"}}},
					},
				)
				require.NoError(t, err)
				return dc
			}(),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.validateDiscoveryConfigIntegration(tc.dc)
			if tc.wantErr {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}

	const knownIntegrationFieldCount = 4

	typeCount := countSpecIntegrationFields(reflect.TypeFor[discoveryconfig.Spec](), make(map[reflect.Type]bool))
	require.Equal(t, knownIntegrationFieldCount, typeCount,
		"DiscoveryConfig.Spec has %d Integration string fields; expected %d — "+
			"add validation in validateDiscoveryConfig and update knownIntegrationFieldCount",
		typeCount, knownIntegrationFieldCount,
	)

	spec := discoveryconfig.Spec{DiscoveryGroup: cloudGroup}
	populateWithOneElem(reflect.ValueOf(&spec).Elem(), 0)
	walkValueIntegrationFields(reflect.ValueOf(&spec).Elem(), "Spec", func(_ string, fv reflect.Value) {
		fv.SetString("my-integration")
	})

	baseDC := &discoveryconfig.DiscoveryConfig{Spec: spec}
	require.NoError(t, svc.validateDiscoveryConfigIntegration(baseDC), "fully-populated DC should pass validation")

	specV := reflect.ValueOf(baseDC).Elem().FieldByName("Spec")
	var paths []string
	walkValueIntegrationFields(specV, "Spec", func(path string, _ reflect.Value) {
		paths = append(paths, path)
	})
	require.Len(t, paths, knownIntegrationFieldCount,
		"baseDC exposes %d Integration fields; expected %d — populate the missing fields in baseDC above",
		len(paths), knownIntegrationFieldCount,
	)

	for i, path := range paths {
		t.Run(path, func(t *testing.T) {
			dc := baseDC.Clone()
			idx := 0
			walkValueIntegrationFields(reflect.ValueOf(dc).Elem().FieldByName("Spec"), "Spec", func(_ string, fv reflect.Value) {
				if idx == i {
					fv.SetString("")
				}
				idx++
			})
			require.True(t, trace.IsBadParameter(svc.validateDiscoveryConfigIntegration(dc)),
				"validateDiscoveryConfig should reject DC with %s cleared", path)
		})
	}
}

func walkValueIntegrationFields(v reflect.Value, prefix string, fn func(path string, fv reflect.Value)) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := range v.Len() {
			walkValueIntegrationFields(v.Index(i), fmt.Sprintf("%s[%d]", prefix, i), fn)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			fv := v.Field(i)
			path := prefix + "." + f.Name
			if strings.Contains(strings.ToLower(f.Name), "integration") && f.Type.Kind() == reflect.String {
				fn(path, fv)
			} else {
				walkValueIntegrationFields(fv, path, fn)
			}
		}
	}
}

func populateWithOneElem(v reflect.Value, depth int) {
	if depth > 10 || v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Slice:
			if fv.Len() == 0 {
				et := fv.Type().Elem()
				var elem reflect.Value
				if et.Kind() == reflect.Pointer {
					p := reflect.New(et.Elem())
					populateWithOneElem(p.Elem(), depth+1)
					elem = p
				} else {
					elem = reflect.New(et).Elem()
					populateWithOneElem(elem, depth+1)
				}
				fv.Set(reflect.Append(fv, elem))
			}
		case reflect.Pointer:
			if fv.IsNil() {
				p := reflect.New(fv.Type().Elem())
				fv.Set(p)
				populateWithOneElem(p.Elem(), depth+1)
			}
		case reflect.Struct:
			populateWithOneElem(fv, depth+1)
		}
	}
}

func countSpecIntegrationFields(t reflect.Type, visited map[reflect.Type]bool) int {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return 0
	}
	if visited[t] {
		return 0
	}
	visited[t] = true

	count := 0
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if strings.HasPrefix(f.Name, "XXX_") {
			continue
		}
		if strings.Contains(strings.ToLower(f.Name), "integration") && f.Type.Kind() == reflect.String {
			count++
		} else {
			count += countSpecIntegrationFields(f.Type, visited)
		}
	}
	return count
}
