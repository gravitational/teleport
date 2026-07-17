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
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	convert "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/aws"
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
			// The role allows the read so the rejection can only come from
			// the deny rule winning; a deny-only role would be rejected even
			// if deny rules were ignored.
			Name: "denied access to read discovery configs",
			Role: types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: []types.Rule{{
					Resources: []string{types.KindDiscoveryConfig},
					Verbs:     []string{types.VerbRead},
				}}},
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
				// Ten stored configs against a page size of five must fill
				// the page and report a next page.
				resp, err := resourceSvc.ListDiscoveryConfigs(ctx, discoveryconfigpb.ListDiscoveryConfigsRequest_builder{
					PageSize:  5,
					NextToken: "",
				}.Build())
				if err != nil {
					return err
				}
				if got := len(resp.GetDiscoveryConfigs()); got != 5 {
					return trace.BadParameter("expected a full page of 5 discovery configs, got %d", got)
				}
				if resp.GetNextKey() == "" {
					return trace.BadParameter("expected a next page key with more configs stored than the page size")
				}
				return nil
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
				if _, err := resourceSvc.DeleteAllDiscoveryConfigs(ctx, &discoveryconfigpb.DeleteAllDiscoveryConfigsRequest{}); err != nil {
					return err
				}
				remaining, _, err := localClient.ListDiscoveryConfigs(ctx, 0, "")
				if err != nil {
					return err
				}
				if len(remaining) != 0 {
					return trace.BadParameter("expected the store emptied, %d discovery configs remain", len(remaining))
				}
				return nil
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

func TestDiscoveryConfigWritesDiscardSubKind(t *testing.T) {
	t.Parallel()

	ctx, localClient, resourceSvc := initSvc(t, "test-cluster")
	writeCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{{
			Resources: []string{types.KindDiscoveryConfig},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		}}},
	}, localClient)

	for _, subKind := range []string{"future-subkind", discoveryconfig.SubKindStaticSnapshot} {
		t.Run(subKind, func(t *testing.T) {
			dc, err := discoveryconfig.NewDiscoveryConfig(
				header.Metadata{Name: uuid.NewString()},
				discoveryconfig.Spec{
					DiscoveryGroup: "some-group",
					AWS: []types.AWSMatcher{{
						Types:   []string{types.AWSMatcherEC2},
						Regions: []string{"us-east-1"},
						Params: &types.InstallerParams{
							JoinToken: "token-name",
							HTTPProxySettings: &types.HTTPProxySettings{
								HTTPSProxy: "http://user:password@proxy.example.com",
							},
						},
					}},
				},
			)
			require.NoError(t, err)
			wantParams := dc.Spec.AWS[0].Params
			dc.SetSubKind(subKind)

			created, err := resourceSvc.CreateDiscoveryConfig(writeCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{
				DiscoveryConfig: convert.ToProto(dc),
			}.Build())
			require.NoError(t, err)
			require.Empty(t, created.GetHeader().GetSubKind())
			require.Equal(t, wantParams, created.GetSpec().GetAws()[0].Params)

			dc.SetSubKind(subKind)
			updated, err := resourceSvc.UpdateDiscoveryConfig(writeCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{
				DiscoveryConfig: convert.ToProto(dc),
			}.Build())
			require.NoError(t, err)
			require.Empty(t, updated.GetHeader().GetSubKind())
			require.Equal(t, wantParams, updated.GetSpec().GetAws()[0].Params)

			dc.SetSubKind(subKind)
			upserted, err := resourceSvc.UpsertDiscoveryConfig(writeCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{
				DiscoveryConfig: convert.ToProto(dc),
			}.Build())
			require.NoError(t, err)
			require.Empty(t, upserted.GetHeader().GetSubKind())
			require.Equal(t, wantParams, upserted.GetSpec().GetAws()[0].Params)
		})
	}
}

func TestStaticSnapshotPublication(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)

	before := time.Now()
	got, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "configured-group"))
	require.NoError(t, err)
	after := time.Now()
	echo, err := convert.FromProtoWithSubKind(got)
	require.NoError(t, err)
	require.True(t, echo.IsStaticSnapshot())
	require.Equal(t, name, echo.GetName())
	require.Equal(t, types.OriginConfigFile, echo.Origin())
	require.False(t, echo.Expiry().Before(before.Add(discoveryconfig.StaticSnapshotTTL)))
	require.False(t, echo.Expiry().After(after.Add(discoveryconfig.StaticSnapshotTTL)))
	// The write echo goes to a Discovery Service identity and must not carry
	// the matcher inventory back to it.
	require.Empty(t, echo.GetDiscoveryGroup())
	require.Empty(t, echo.Spec.AWS)

	// The snapshot must live in the isolated range only: the regular range is
	// what Discovery Services list and watch for dynamic matchers.
	regular, _, err := cfg.Backend.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, regular)
	stored, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "configured-group", stored.GetDiscoveryGroup())
	require.Len(t, stored.Spec.AWS, 1)

	// Production Discovery Services may carry Discovery as an additional role
	// on their instance identity.
	_, err = resourceSvc.UpsertDiscoveryConfig(
		authorizerForInstanceDiscoveryOwner(ctx, clusterName, serverID), staticSnapshotUpsertRequest(t, serverID, "configured-group"))
	require.NoError(t, err)

	// A service with no discovery group still publishes: only the
	// static-snapshot subkind relaxes the group requirement.
	groupless, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig(serverID, discoveryconfig.Spec{})
	require.NoError(t, err)
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(groupless)}.Build())
	require.NoError(t, err)
}

func TestStaticSnapshotRenewalAdvancesExpiryAndPreservesStatus(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testStaticSnapshotRenewalAdvancesExpiryAndPreservesStatus)
}

func testStaticSnapshotRenewalAdvancesExpiryAndPreservesStatus(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)

	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group-one"))
	require.NoError(t, err)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{
		DiscoveredResources: 42,
		ServerStatus: map[string]*discoveryconfigpb.DiscoveryStatusServer{
			serverID: discoveryconfigpb.DiscoveryStatusServer_builder{}.Build(),
		},
	}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build())
	require.NoError(t, err)

	before, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	time.Sleep(time.Minute)
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group-two"))
	require.NoError(t, err)

	after, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, before.Expiry().Add(time.Minute), after.Expiry(), "renewal must advance the Auth-owned expiry")
	require.Equal(t, "group-two", after.GetDiscoveryGroup(), "renewal must replace the spec inventory")
	require.Equal(t, before.Status.DiscoveredResources, after.Status.DiscoveredResources, "renewal must preserve the report counters")
	require.Contains(t, after.Status.ServerStatus, serverID, "renewal must preserve the owner's server report")
}

func TestStaticSnapshotPublicationValidation(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)

	// Publication is fail-closed: unsanitized installer params are rejected
	// rather than silently stripped.
	req := staticSnapshotUpsertRequest(t, serverID, "group")
	req.GetDiscoveryConfig().GetSpec().GetAws()[0].Params = &types.InstallerParams{JoinToken: "secret"}
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, req)
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	// A missing spec is malformed publication, not an empty inventory update.
	req = staticSnapshotUpsertRequest(t, serverID, "group")
	req.GetDiscoveryConfig().ClearSpec()
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, req)
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "spec is missing")

	// Owners cannot publish under another server's name.
	req = staticSnapshotUpsertRequest(t, serverID, "group")
	req.GetDiscoveryConfig().GetHeader().GetMetadata().SetName(discoveryconfig.StaticSnapshotName(uuid.NewString()))
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, req)
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	// A Discovery Service identity keeps no generic write access outside the
	// snapshot publication path.
	regularWrite, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "regular"}, discoveryconfig.Spec{DiscoveryGroup: "group"})
	require.NoError(t, err)
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(regularWrite)}.Build())
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	// The stored-size cap covers the whole record.
	req = staticSnapshotUpsertRequest(t, serverID, strings.Repeat("x", discoveryconfig.MaxStaticSnapshotSize))
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, req)
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)

	// A grandfathered regular config occupying the owner name blocks publication.
	regular, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: discoveryconfig.StaticSnapshotName(serverID)},
		discoveryconfig.Spec{DiscoveryGroup: "legacy"},
	)
	require.NoError(t, err)
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.True(t, trace.IsAlreadyExists(err), "got %v", err)
}

func TestStaticSnapshotStatusOwnershipAndCASMerges(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group-one"))
	require.NoError(t, err)

	before, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{
		State:               discoveryconfigpb.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING,
		DiscoveredResources: 42,
		ServerStatus: map[string]*discoveryconfigpb.DiscoveryStatusServer{
			serverID: discoveryconfigpb.DiscoveryStatusServer_builder{}.Build(),
		},
	}.Build()
	updated, err := resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build())
	require.NoError(t, err)
	updatedInternal, err := convert.FromProtoWithSubKind(updated)
	require.NoError(t, err)
	require.Equal(t, before.Expiry(), updatedInternal.Expiry(), "report updates must not renew expiry")
	require.Equal(t, uint64(42), updatedInternal.Status.DiscoveredResources)
	require.Empty(t, updatedInternal.GetDiscoveryGroup(), "write echoes to Discovery identities must not carry inventory")
	afterReport, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "group-one", afterReport.GetDiscoveryGroup(), "report updates must not touch the spec inventory")
	require.Equal(t, before.Spec, afterReport.Spec, "report updates must not touch the spec inventory")
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForInstanceDiscoveryOwner(ctx, clusterName, serverID),
		discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build(),
	)
	require.NoError(t, err)

	// A foreign Discovery Service gets the same NotFound an unoccupied name
	// produces: the status RPC must not disclose which snapshots exist.
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForDiscoveryOwner(ctx, clusterName, uuid.NewString()),
		discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build(),
	)
	require.True(t, trace.IsNotFound(err), "got %v", err)
	foreign := discoveryconfigpb.DiscoveryConfigStatus_builder{
		ServerStatus: map[string]*discoveryconfigpb.DiscoveryStatusServer{
			"foreign": discoveryconfigpb.DiscoveryStatusServer_builder{}.Build(),
		},
	}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: foreign}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	// A publication racing a report update retries from the latest report.
	baseSnapshotBackend := cfg.StaticSnapshotBackend
	raceCfg := cfg
	raceCfg.StaticSnapshotBackend = &snapshotUpdateHookBackend{
		StaticSnapshotDiscoveryConfigs: baseSnapshotBackend,
		hook: func() error {
			current, err := baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
			if err != nil {
				return err
			}
			current.Status.DiscoveredResources = 99
			_, err = baseSnapshotBackend.ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx, current)
			return err
		},
	}
	resourceSvc = newService(t, raceCfg)
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group-two"))
	require.NoError(t, err)
	stored, err := baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "group-two", stored.GetDiscoveryGroup())
	require.Equal(t, uint64(99), stored.Status.DiscoveredResources)

	// A report update racing an inventory renewal retries from the latest inventory.
	raceCfg = cfg
	raceCfg.StaticSnapshotBackend = &snapshotUpdateHookBackend{
		StaticSnapshotDiscoveryConfigs: baseSnapshotBackend,
		hook: func() error {
			current, err := baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
			if err != nil {
				return err
			}
			current.Spec.DiscoveryGroup = "group-three"
			_, err = baseSnapshotBackend.ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx, current)
			return err
		},
	}
	resourceSvc = newService(t, raceCfg)
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build())
	require.NoError(t, err)
	stored, err = baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "group-three", stored.GetDiscoveryGroup())
	require.Equal(t, uint64(42), stored.Status.DiscoveredResources)
}

func TestStaticSnapshotRejectsOversizedReport(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)

	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)

	// The stored-size cap is validated against the merged record, so an
	// oversized report fails loudly instead of consuming the space the owner
	// needs to renew its inventory.
	errorMessage := strings.Repeat("x", discoveryconfig.MaxStaticSnapshotSize)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{ErrorMessage: &errorMessage}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build())
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)

	stored, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Nil(t, stored.Status.ErrorMessage, "rejected report must not replace stored status")

	// Inventory renewal keeps working after the rejected report.
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)
}

// TestStaticSnapshotReadThenWriteContract covers get-then-mutate flows (web UI
// edits, tctl edit): a snapshot is readable through the legacy Get fallback,
// and every generic write against its name fails with the stateless
// reserved-name rejection, not validation noise, and not NotFound.
func TestStaticSnapshotReadThenWriteContract(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, localClient, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "snapshot-group"))
	require.NoError(t, err)

	userCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbUpdate},
	}}}}, localClient)
	got, err := resourceSvc.GetDiscoveryConfig(withClientVersion(userCtx, "19.0.0"), discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.NoError(t, err)

	_, err = resourceSvc.UpdateDiscoveryConfig(userCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: got}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "reserved for static snapshot")
	_, err = resourceSvc.UpsertDiscoveryConfig(userCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: got}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "reserved for static snapshot")
	_, err = resourceSvc.CreateDiscoveryConfig(userCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: got}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "reserved for static snapshot")
}

func TestStaticSnapshotClientSupported(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		supported bool
		wantErr   bool
	}{
		{name: "older stable", version: "18.5.0"},
		{name: "v19 prealpha before feature", version: "19.0.0-prealpha.1"},
		{name: "v19 current prealpha", version: "19.0.0-prealpha.2"},
		{name: "v19 release candidate", version: "19.0.0-rc.1"},
		{name: "v19 stable", version: "19.0.0", supported: true},
		{name: "future prerelease", version: "20.0.0-prealpha.1", supported: true},
		{name: "invalid", version: "not-semver", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := withClientVersion(t.Context(), tt.version)
			supported, err := staticSnapshotClientSupported(ctx)
			if tt.wantErr {
				require.True(t, trace.IsBadParameter(err),
					"a version that does not parse must be a BadParameter, got %v", err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.supported, supported)
		})
	}

	supported, err := staticSnapshotClientSupported(t.Context())
	require.NoError(t, err)
	require.False(t, supported,
		"clients that do not report a version must fail closed")
}

// TestStaticSnapshotPublicationDoesNotResurrectDeletedStatus covers the
// retry-loop race where an update attempt observes an existing record
// (merging its status into the working copy), loses the CAS race, and the
// record is deleted before the next iteration: the create that follows must
// publish a fresh record, not the deleted record's status.
func TestStaticSnapshotPublicationDoesNotResurrectDeletedStatus(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testStaticSnapshotPublicationDoesNotResurrectDeletedStatus)
}

func testStaticSnapshotPublicationDoesNotResurrectDeletedStatus(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, _, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)

	stale, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig(serverID, discoveryconfig.Spec{DiscoveryGroup: "stale-group"})
	require.NoError(t, err)
	stale.Status.DiscoveredResources = 7
	stale.SetRevision("stale-revision")

	race := &snapshotDeletionRaceBackend{stale: stale}
	cfg.StaticSnapshotBackend = race
	resourceSvc := newService(t, cfg)

	// The lost CAS race parks the retry loop on its backoff timer; the
	// bubble's fake clock advances past it without real sleeping.
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "fresh-group"))
	require.NoError(t, err)
	require.NotNil(t, race.created)
	require.Equal(t, "fresh-group", race.created.GetDiscoveryGroup())
	require.Zero(t, race.created.Status.DiscoveredResources,
		"a fresh publication must not resurrect status from a deleted record")
	require.Empty(t, race.created.Status.ServerStatus)
	require.Empty(t, race.created.GetRevision())
}

// snapshotDeletionRaceBackend scripts the deletion race: the first Get
// returns a pre-existing record, the update attempt against it loses the CAS
// race, and every later Get reports the record gone so the retry takes the
// create branch.
type snapshotDeletionRaceBackend struct {
	services.StaticSnapshotDiscoveryConfigs
	stale   *discoveryconfig.DiscoveryConfig
	gets    int
	created *discoveryconfig.DiscoveryConfig
}

func (b *snapshotDeletionRaceBackend) GetStaticSnapshotDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	b.gets++
	if b.gets == 1 {
		return b.stale, nil
	}
	return nil, trace.NotFound("static snapshot discovery config %q not found", name)
}

func (b *snapshotDeletionRaceBackend) ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.CompareFailed("concurrent write")
}

func (b *snapshotDeletionRaceBackend) CreateStaticSnapshotDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	b.created = dc.Clone()
	return dc, nil
}

// TestStaticSnapshotPublicationRetryExhaustion pins the error category of an
// exhausted publication CAS loop: CompareFailed, matching the status loops,
// with LimitExceeded reserved for the stored-size cap. A publisher relies on
// the distinction to tell write contention (retry soon) from an oversized
// record (shrink first).
func TestStaticSnapshotPublicationRetryExhaustion(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testStaticSnapshotPublicationRetryExhaustion)
}

func testStaticSnapshotPublicationRetryExhaustion(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)
	before, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)

	baseSnapshotBackend := cfg.StaticSnapshotBackend
	failingBackend := &alwaysCompareFailedSnapshotBackend{StaticSnapshotDiscoveryConfigs: baseSnapshotBackend}
	cfg.StaticSnapshotBackend = failingBackend
	resourceSvc = newService(t, cfg)

	// The bubble's fake clock walks the retry loop through its backoff
	// timers, so exhaustion needs no goroutine or clock choreography.
	_, err = resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "contended-group"))
	require.True(t, trace.IsCompareFailed(err), "got %v", err)
	require.False(t, trace.IsLimitExceeded(err),
		"contention exhaustion must not share the oversized-record category")
	require.Equal(t, discoveryConfigWriteAttempts, failingBackend.attempts)

	after, err := baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, before.Spec, after.Spec, "exhausted retries must not alter the stored spec")
	require.Equal(t, before.Expiry(), after.Expiry(), "exhausted retries must not alter the stored expiry")
	require.Equal(t, before.GetRevision(), after.GetRevision(), "exhausted retries must not alter the stored revision")
}

// TestStaticSnapshotStatusRetryExhaustion pins the bounded status CAS loop:
// exactly discoveryConfigWriteAttempts attempts, a CompareFailed terminal
// category, and an unchanged stored record afterward.
func TestStaticSnapshotStatusRetryExhaustion(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testStaticSnapshotStatusRetryExhaustion)
}

func testStaticSnapshotStatusRetryExhaustion(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)
	before, err := cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)

	baseSnapshotBackend := cfg.StaticSnapshotBackend
	failingBackend := &alwaysCompareFailedSnapshotBackend{StaticSnapshotDiscoveryConfigs: baseSnapshotBackend}
	cfg.StaticSnapshotBackend = failingBackend
	resourceSvc = newService(t, cfg)

	// The bubble's fake clock walks the retry loop through its backoff
	// timers, so exhaustion needs no goroutine or clock choreography.
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{
		Name:   name,
		Status: discoveryconfigpb.DiscoveryConfigStatus_builder{DiscoveredResources: 42}.Build(),
	}.Build())
	require.True(t, trace.IsCompareFailed(err), "got %v", err)
	require.Equal(t, discoveryConfigWriteAttempts, failingBackend.attempts)

	after, err := baseSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, before.Spec, after.Spec, "exhausted retries must not alter the stored spec")
	require.Equal(t, before.Status.DiscoveredResources, after.Status.DiscoveredResources, "exhausted retries must not alter the stored report counters")
	require.Equal(t, before.Status.ServerStatus, after.Status.ServerStatus, "exhausted retries must not alter the stored server reports")
	require.Equal(t, before.Expiry(), after.Expiry(), "exhausted retries must not alter the stored expiry")
	require.Equal(t, before.GetRevision(), after.GetRevision(), "exhausted retries must not alter the stored revision")
}

// TestStaticSnapshotStatusRetryCancellation pins that context cancellation
// wins over the retry backoff timer: the RPC returns the context error after
// exactly one attempt instead of burning the remaining retries.
func TestStaticSnapshotStatusRetryCancellation(t *testing.T) {
	t.Parallel()
	synctest.Test(t, testStaticSnapshotStatusRetryCancellation)
}

func testStaticSnapshotStatusRetryCancellation(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)

	failingBackend := &alwaysCompareFailedSnapshotBackend{StaticSnapshotDiscoveryConfigs: cfg.StaticSnapshotBackend}
	cfg.StaticSnapshotBackend = failingBackend
	resourceSvc = newService(t, cfg)
	canceledCtx, cancel := context.WithCancel(ownerCtx)
	result := make(chan error, 1)
	go func() {
		_, err := resourceSvc.UpdateDiscoveryConfigStatus(canceledCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{
			Name: name,
		}.Build())
		result <- err
	}()
	// Wait until the RPC is durably parked on the first retry timer,
	// then cancel while it waits: the cancellation must win over the
	// timer instead of the clock advancing past it.
	synctest.Wait()
	cancel()
	err = <-result
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, failingBackend.attempts, "cancellation must stop before the first retry")
}

type alwaysCompareFailedSnapshotBackend struct {
	services.StaticSnapshotDiscoveryConfigs
	attempts int
}

func (b *alwaysCompareFailedSnapshotBackend) ConditionalUpdateStaticSnapshotDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	b.attempts++
	return nil, trace.CompareFailed("concurrent write")
}

func TestStaticSnapshotGenericReadAndDeletion(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, localClient, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "snapshot-group"))
	require.NoError(t, err)

	userCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbRead, types.VerbList, types.VerbDelete},
	}}}}, localClient)

	// Clients that can decode static snapshots receive them from the legacy
	// Get RPC.
	got, err := resourceSvc.GetDiscoveryConfig(withClientVersion(userCtx, "19.0.0"), discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.NoError(t, err)
	require.Equal(t, discoveryconfig.SubKindStaticSnapshot, got.GetHeader().GetSubKind())
	require.Equal(t, "snapshot-group", got.GetSpec().GetDiscoveryGroup())

	// Clients that predate the static-snapshot subkind (or report no
	// version at all) would fail decoding a group-less snapshot client-side,
	// so they receive NotFound.
	_, err = resourceSvc.GetDiscoveryConfig(withClientVersion(userCtx, "18.4.0"), discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)
	_, err = resourceSvc.GetDiscoveryConfig(userCtx, discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)

	// A version that does not parse surfaces as BadParameter, consistent
	// with the regular-config downgrade path, instead of folding into the
	// NotFound reserved for too-old clients.
	_, err = resourceSvc.GetDiscoveryConfig(withClientVersion(userCtx, "not-semver"), discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	// The owning Discovery Service reads its own snapshot inventory-stripped
	// (envelope and status only) so read-modify-write status reporting can
	// merge against stored history without any channel carrying matchers
	// back to a service.
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{DiscoveredResources: 7}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name, Status: report}.Build())
	require.NoError(t, err)
	ownerGot, err := resourceSvc.GetDiscoveryConfig(ownerCtx, discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.NoError(t, err)
	require.Equal(t, discoveryconfig.SubKindStaticSnapshot, ownerGot.GetHeader().GetSubKind())
	require.Empty(t, ownerGot.GetSpec().GetDiscoveryGroup(), "owner reads must not carry inventory")
	require.Empty(t, ownerGot.GetSpec().GetAws(), "owner reads must not carry inventory")
	require.Equal(t, uint64(7), ownerGot.GetStatus().GetDiscoveredResources(), "owner reads must carry stored status for merge loops")

	// A foreign Discovery Service gets the unoccupied-name NotFound.
	_, err = resourceSvc.GetDiscoveryConfig(
		authorizerForDiscoveryOwner(ctx, clusterName, uuid.NewString()),
		discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)

	_, err = resourceSvc.DeleteDiscoveryConfig(userCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	// Bulk deletion clears the regular store but must not reach the isolated
	// snapshot record.
	doomed, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "regular-config"}, discoveryconfig.Spec{DiscoveryGroup: "regular-group"})
	require.NoError(t, err)
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, doomed)
	require.NoError(t, err)
	_, err = resourceSvc.DeleteAllDiscoveryConfigs(userCtx, &discoveryconfigpb.DeleteAllDiscoveryConfigsRequest{})
	require.NoError(t, err)
	_, err = cfg.Backend.GetDiscoveryConfig(ctx, doomed.GetName())
	require.True(t, trace.IsNotFound(err), "got %v", err)
	_, err = cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)

	// A pre-existing regular config with the same name wins the legacy Get
	// without affecting the isolated snapshot record.
	regular, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: name}, discoveryconfig.Spec{DiscoveryGroup: "regular-group"})
	require.NoError(t, err)
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	got, err = resourceSvc.GetDiscoveryConfig(withClientVersion(userCtx, "19.0.0"), discoveryconfigpb.GetDiscoveryConfigRequest_builder{Name: name}.Build())
	require.NoError(t, err)
	require.Empty(t, got.GetHeader().GetSubKind())
	require.Equal(t, "regular-group", got.GetSpec().GetDiscoveryGroup())
	_, err = cfg.StaticSnapshotBackend.GetStaticSnapshotDiscoveryConfig(ctx, name)
	require.NoError(t, err)
}

// TestStaticSnapshotExistenceOnlyDisclosedToReaders pins the anti-oracle
// contract: whether a snapshot occupies a reserved name is only revealed
// (via the owner-managed AccessDenied on delete attempts) to callers holding
// the read verb, who could learn it through Get anyway. Every other caller
// receives exactly the unoccupied-name responses; Create, Update, and Upsert
// reject every reserved name statelessly, so no RPC works as an existence
// probe.
func TestStaticSnapshotExistenceOnlyDisclosedToReaders(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, localClient, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.StaticSnapshotName(serverID)
	_, err := resourceSvc.UpsertDiscoveryConfig(ownerCtx, staticSnapshotUpsertRequest(t, serverID, "group"))
	require.NoError(t, err)

	writeOnlyCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbCreate, types.VerbUpdate, types.VerbDelete},
	}}}}, localClient)
	payload, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: name}, discoveryconfig.Spec{DiscoveryGroup: "group"})
	require.NoError(t, err)

	_, err = resourceSvc.DeleteDiscoveryConfig(writeOnlyCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: name}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)
	_, err = resourceSvc.CreateDiscoveryConfig(writeOnlyCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(payload)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeOnlyCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(payload)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpdateDiscoveryConfig(writeOnlyCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(payload)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	// The snapshot store is never consulted for names outside the reserved
	// namespace: a snapshot-backend outage must not turn an ordinary delete
	// miss into an internal error.
	cfg.StaticSnapshotBackend = failingSnapshotBackend{}
	resourceSvc = newService(t, cfg)
	_, err = resourceSvc.DeleteDiscoveryConfig(writeOnlyCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: "no-such-config"}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)
}

type failingSnapshotBackend struct {
	services.StaticSnapshotDiscoveryConfigs
}

func (failingSnapshotBackend) GetStaticSnapshotDiscoveryConfig(context.Context, string) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.Errorf("the snapshot store must not be consulted")
}

func TestRegularReservedAndLegacyNames(t *testing.T) {
	t.Parallel()
	ctx, localClient, resourceSvc, cfg := initSvcWithConfig(t, "test-cluster")
	writeCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbCreate, types.VerbUpdate, types.VerbDelete},
	}}}}, localClient)

	reserved, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: discoveryconfig.StaticSnapshotName(uuid.NewString())},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	// Every generic write to a reserved name is rejected by the stateless
	// name-shape check with guidance to use a different name. Nothing is
	// read from the backend, so the answer cannot depend on occupancy, and
	// an IaC reconcile loop is not sent chasing a phantom resource
	// (BadParameter, never AlreadyExists).
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "recreate it under a different name")
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.DeleteDiscoveryConfig(writeCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: reserved.GetName()}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)

	// A reserved-shaped regular config predating the reservation keeps its
	// read/delete contract, but its spec is frozen: the same stateless
	// rejection answers every write, occupied or not, and the rejected
	// writes leave the stored record untouched.
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, reserved)
	require.NoError(t, err)
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	reserved.Spec.DiscoveryGroup = "updated-group"
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	stored, err := cfg.Backend.GetDiscoveryConfig(ctx, reserved.GetName())
	require.NoError(t, err)
	require.Equal(t, "group", stored.GetDiscoveryGroup(), "rejected writes must leave the frozen spec untouched")

	// The migration path the rejection error spells out: deletion still
	// works, exactly once, and the name then keeps the unoccupied contract.
	_, err = resourceSvc.DeleteDiscoveryConfig(writeCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: reserved.GetName()}.Build())
	require.NoError(t, err)
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "recreate it under a different name")
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(reserved)}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.DeleteDiscoveryConfig(writeCtx, discoveryconfigpb.DeleteDiscoveryConfigRequest_builder{Name: reserved.GetName()}.Build())
	require.True(t, trace.IsNotFound(err), "got %v", err)

	legacy, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "static-snapshot-aws-prod"},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, discoveryconfigpb.CreateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(legacy)}.Build())
	require.NoError(t, err)
	legacy.Spec.DiscoveryGroup = "updated"
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, discoveryconfigpb.UpdateDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(legacy)}.Build())
	require.NoError(t, err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(legacy)}.Build())
	require.NoError(t, err)
}

// TestRegularStatusUpdateDoesNotClobberConcurrentSpecEdit pins the regular
// status path's revision-checked write: a status report whose working copy
// was fetched before a concurrent matcher edit must lose the CAS race and
// retry from the edited config, not silently revert the user's edit.
func TestRegularStatusUpdateDoesNotClobberConcurrentSpecEdit(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, _, cfg := initSvcWithConfig(t, clusterName)

	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "racy-config"},
		discoveryconfig.Spec{DiscoveryGroup: "original-group"},
	)
	require.NoError(t, err)
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, dc)
	require.NoError(t, err)

	baseBackend := cfg.Backend
	racingBackend := &regularUpdateHookBackend{DiscoveryConfigsInternal: baseBackend}
	racingBackend.beforeConditionalUpdate = func() error {
		current, err := baseBackend.GetDiscoveryConfig(ctx, dc.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		current.Spec.DiscoveryGroup = "edited-group"
		_, err = baseBackend.UpdateDiscoveryConfig(ctx, current)
		return trace.Wrap(err)
	}
	cfg.Backend = racingBackend
	resourceSvc := newService(t, cfg)

	report := discoveryconfigpb.DiscoveryConfigStatus_builder{DiscoveredResources: 42}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForDiscoveryOwner(ctx, clusterName, serverID),
		discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: dc.GetName(), Status: report}.Build(),
	)
	require.NoError(t, err)

	stored, err := baseBackend.GetDiscoveryConfig(ctx, dc.GetName())
	require.NoError(t, err)
	require.Equal(t, "edited-group", stored.GetDiscoveryGroup(),
		"a stale status write must not revert a concurrent spec edit")
	require.Equal(t, uint64(42), stored.Status.DiscoveredResources)
}

func TestDeletedGrandfatheredConfigStatusReturnsNotFound(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc, cfg := initSvcWithConfig(t, clusterName)
	name := discoveryconfig.StaticSnapshotName(uuid.NewString())

	regular, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: name},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	_, err = cfg.Backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	require.NoError(t, cfg.Backend.DeleteDiscoveryConfig(ctx, name))

	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForDiscoveryOwner(ctx, clusterName, serverID),
		discoveryconfigpb.UpdateDiscoveryConfigStatusRequest_builder{Name: name}.Build(),
	)
	require.True(t, trace.IsNotFound(err), "got %v", err)
}

type regularUpdateHookBackend struct {
	services.DiscoveryConfigsInternal
	beforeConditionalUpdate func() error
}

func (b *regularUpdateHookBackend) ConditionalUpdateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if b.beforeConditionalUpdate != nil {
		beforeConditionalUpdate := b.beforeConditionalUpdate
		b.beforeConditionalUpdate = nil
		if err := beforeConditionalUpdate(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return b.DiscoveryConfigsInternal.ConditionalUpdateDiscoveryConfig(ctx, dc)
}

// staticSnapshotUpsertRequest builds the publication request a Discovery
// Service sends: its own snapshot resource through the generic upsert RPC.
func staticSnapshotUpsertRequest(t *testing.T, serverID, group string) *discoveryconfigpb.UpsertDiscoveryConfigRequest {
	t.Helper()
	dc, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig(serverID, discoveryconfig.Spec{
		DiscoveryGroup: group,
		AWS: []types.AWSMatcher{{
			Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
		}},
	})
	require.NoError(t, err)
	return discoveryconfigpb.UpsertDiscoveryConfigRequest_builder{DiscoveryConfig: convert.ToProto(dc)}.Build()
}

// withClientVersion stamps the incoming gRPC client version metadata the
// service's version gate reads.
func withClientVersion(ctx context.Context, version string) context.Context {
	return grpcmetadata.NewIncomingContext(ctx, grpcmetadata.Pairs(metadata.VersionKey, version))
}

type snapshotUpdateHookBackend struct {
	services.StaticSnapshotDiscoveryConfigs
	hook func() error
}

func (b *snapshotUpdateHookBackend) ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if b.hook != nil {
		hook := b.hook
		b.hook = nil
		if err := hook(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return b.StaticSnapshotDiscoveryConfigs.ConditionalUpdateStaticSnapshotDiscoveryConfig(ctx, dc)
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

func authorizerForDiscoveryOwner(ctx context.Context, clusterName, serverID string) context.Context {
	return authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:        types.RoleDiscovery,
		Username:    serverID,
		ClusterName: clusterName,
		Identity: tlsca.Identity{
			Username:    serverID + "." + clusterName,
			SystemRoles: []string{string(types.RoleDiscovery)},
			Groups:      []string{string(types.RoleDiscovery)},
		},
	})
}

func authorizerForInstanceDiscoveryOwner(ctx context.Context, clusterName, serverID string) context.Context {
	return authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:                  types.RoleInstance,
		AdditionalSystemRoles: types.SystemRoles{types.RoleDiscovery},
		Username:              serverID,
		ClusterName:           clusterName,
		Identity: tlsca.Identity{
			Username:    serverID + "." + clusterName,
			SystemRoles: []string{string(types.RoleInstance), string(types.RoleDiscovery)},
			Groups:      []string{string(types.RoleInstance), string(types.RoleDiscovery)},
		},
	})
}

type localClient interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	CreateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	ListDiscoveryConfigs(ctx context.Context, pageSize int, pageToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
	services.Presence
}

func initSvc(t *testing.T, clusterName string) (context.Context, localClient, *Service) {
	ctx, lc, svc, _ := initSvcWithConfig(t, clusterName)
	return ctx, lc, svc
}

// newService constructs a Service through the public constructor, failing the
// test on an invalid config.
func newService(t *testing.T, cfg ServiceConfig) *Service {
	t.Helper()
	svc, err := NewService(cfg)
	require.NoError(t, err)
	return svc
}

// initSvcWithConfig additionally returns the ServiceConfig the service was
// built from. Tests that need doubles (failing backends, fake clocks) rebuild
// the service through newService with an adjusted copy of that config rather
// than assigning the returned service's unexported fields: construction
// invariants then apply to the doubles, and the tests stay decoupled from the
// Service struct layout.
func initSvcWithConfig(t *testing.T, clusterName string) (context.Context, localClient, *Service, ServiceConfig) {
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
	// synctest bubbles require every goroutine started inside them to exit
	// before the bubble does; close the watcher and backend so tests can run
	// inside synctest.Test.
	t.Cleanup(lockWatcher.Close)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	localResourceService, err := local.NewDiscoveryConfigService(backend)
	require.NoError(t, err)

	emitter := events.NewDiscardEmitter()

	cfg := ServiceConfig{
		Backend:               localResourceService,
		StaticSnapshotBackend: localResourceService,
		Authorizer:            authorizer,
		Emitter:               emitter,
		UsageReporter:         usagereporter.DiscardUsageReporter{},
	}
	resourceSvc := newService(t, cfg)

	return ctx, struct {
		*local.AccessService
		*local.IdentityService
		*local.DiscoveryConfigService
	}{
		AccessService:          roleSvc,
		IdentityService:        userSvc,
		DiscoveryConfigService: localResourceService,
	}, resourceSvc, cfg
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

// TestDowngrade pins the AWS wildcard-region downgrade against incoming
// client-version metadata (the previous form stamped outgoing metadata the
// server never reads, so no version reached the code under test and both
// cases passed vacuously).
func TestDowngrade(t *testing.T) {
	t.Parallel()
	wildcardDC := func() *discoveryconfig.DiscoveryConfig {
		dc, err := discoveryconfig.NewDiscoveryConfig(
			header.Metadata{Name: "dc1"},
			discoveryconfig.Spec{
				DiscoveryGroup: "group1",
				AWS: []types.AWSMatcher{{
					Regions: []string{types.Wildcard},
					Types:   []string{"ec2"},
				}},
			},
		)
		require.NoError(t, err)
		return dc
	}

	t.Run("no downgrade for recent client", func(t *testing.T) {
		downgraded, err := MaybeDowngradeDiscoveryConfig(withClientVersion(t.Context(), "18.5.0"), wildcardDC())
		require.NoError(t, err)
		require.Equal(t, []string{types.Wildcard}, downgraded.Spec.AWS[0].Regions)
	})

	t.Run("downgrade for old client", func(t *testing.T) {
		input := wildcardDC()
		downgraded, err := MaybeDowngradeDiscoveryConfig(withClientVersion(t.Context(), "18.4.2"), input)
		require.NoError(t, err)
		require.Equal(t, []string{aws.AWSGlobalRegion}, downgraded.Spec.AWS[0].Regions,
			"pre-18.5 clients must receive the wildcard region downgraded")
		require.Equal(t, []string{types.Wildcard}, input.Spec.AWS[0].Regions,
			"downgrading must not mutate the source resource")
	})
}
