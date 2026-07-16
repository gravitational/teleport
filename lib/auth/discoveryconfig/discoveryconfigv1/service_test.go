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

func TestDiscoveryConfigWritesDiscardSubKind(t *testing.T) {
	t.Parallel()

	ctx, localClient, resourceSvc := initSvc(t, "test-cluster")
	writeCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{
		Allow: types.RoleConditions{Rules: []types.Rule{{
			Resources: []string{types.KindDiscoveryConfig},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		}}},
	}, localClient)

	for _, subKind := range []string{"future-subkind", discoveryconfig.SubKindSynthetic} {
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

func TestSyntheticDiscoveryConfigPublication(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	request := syntheticPublicationRequest("configured-group")

	before := time.Now()
	got, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, request)
	require.NoError(t, err)
	after := time.Now()
	stored, err := convert.FromProto(got)
	require.NoError(t, err)
	require.True(t, stored.IsSynthetic())
	require.Equal(t, discoveryconfig.SyntheticName(serverID), stored.GetName())
	require.Equal(t, types.OriginConfigFile, stored.Origin())
	require.Empty(t, stored.Spec.DiscoveryGroup)
	require.Equal(t, "configured-group", stored.ConfiguredDiscoveryGroup())
	require.False(t, stored.Expiry().Before(before.Add(discoveryconfig.SyntheticDiscoveryConfigTTL)))
	require.False(t, stored.Expiry().After(after.Add(discoveryconfig.SyntheticDiscoveryConfigTTL)))

	regular, _, err := resourceSvc.backend.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, regular)
	synthetic, _, err := resourceSvc.syntheticBackend.ListSyntheticDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, synthetic, 1)

	// Production Discovery Services may carry Discovery as an additional role
	// on their instance identity.
	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(
		authorizerForInstanceDiscoveryOwner(ctx, clusterName, serverID), request)
	require.NoError(t, err)
}

func TestSyntheticPublicationValidation(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)

	for name, req := range map[string]*discoveryconfigpb.UpsertSyntheticDiscoveryConfigRequest{
		"missing inventory": {},
		"both representations": {
			Synthetic: &discoveryconfigpb.SyntheticDiscoveryConfigStatus{
				Matchers:      &discoveryconfigpb.DiscoveryConfigSpec{},
				MatcherCounts: &discoveryconfigpb.StaticMatcherCounts{},
			},
		},
		"installer params": {
			Synthetic: &discoveryconfigpb.SyntheticDiscoveryConfigStatus{Matchers: &discoveryconfigpb.DiscoveryConfigSpec{
				Aws: []*types.AWSMatcher{{Params: &types.InstallerParams{JoinToken: "secret"}}},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, req)
			require.True(t, trace.IsBadParameter(err), "got %v", err)
		})
	}

	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, &discoveryconfigpb.UpsertSyntheticDiscoveryConfigRequest{
		Synthetic: &discoveryconfigpb.SyntheticDiscoveryConfigStatus{Matchers: &discoveryconfigpb.DiscoveryConfigSpec{
			DiscoveryGroup: strings.Repeat("x", discoveryconfig.SyntheticMatcherDetailBudget+1),
		}},
	})
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)

	regular, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: discoveryconfig.SyntheticName(serverID)},
		discoveryconfig.Spec{DiscoveryGroup: "legacy"},
	)
	require.NoError(t, err)
	_, err = resourceSvc.backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("group"))
	require.True(t, trace.IsAlreadyExists(err), "got %v", err)
}

func TestSyntheticStatusOwnershipAndCASMerges(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.SyntheticName(serverID)
	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("group-one"))
	require.NoError(t, err)

	before, err := resourceSvc.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{
		State:               discoveryconfigpb.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING,
		DiscoveredResources: 42,
		ServerStatus: map[string]*discoveryconfigpb.DiscoveryStatusServer{
			serverID: discoveryconfigpb.DiscoveryStatusServer_builder{}.Build(),
		},
	}.Build()
	updated, err := resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report})
	require.NoError(t, err)
	updatedInternal, err := convert.FromProto(updated)
	require.NoError(t, err)
	require.Equal(t, before.Status.Synthetic, updatedInternal.Status.Synthetic)
	require.Equal(t, before.Expiry(), updatedInternal.Expiry(), "report updates must not renew expiry")
	require.Equal(t, uint64(42), updatedInternal.Status.DiscoveredResources)
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForInstanceDiscoveryOwner(ctx, clusterName, serverID),
		&discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report},
	)
	require.NoError(t, err)

	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForDiscoveryOwner(ctx, clusterName, uuid.NewString()),
		&discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report},
	)
	require.True(t, trace.IsAccessDenied(err), "got %v", err)
	foreign := discoveryconfigpb.DiscoveryConfigStatus_builder{
		ServerStatus: map[string]*discoveryconfigpb.DiscoveryStatusServer{
			"foreign": discoveryconfigpb.DiscoveryStatusServer_builder{}.Build(),
		},
	}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: foreign})
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	// A publication racing a report update retries from the latest report.
	baseSyntheticBackend := resourceSvc.syntheticBackend
	resourceSvc.syntheticBackend = &syntheticUpdateHookBackend{
		SyntheticDiscoveryConfigs: baseSyntheticBackend,
		hook: func() error {
			current, err := baseSyntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
			if err != nil {
				return err
			}
			current.Status.DiscoveredResources = 99
			_, err = baseSyntheticBackend.ConditionalUpdateSyntheticDiscoveryConfig(ctx, current)
			return err
		},
	}
	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("group-two"))
	require.NoError(t, err)
	stored, err := baseSyntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "group-two", stored.ConfiguredDiscoveryGroup())
	require.Equal(t, uint64(99), stored.Status.DiscoveredResources)

	// A report update racing an inventory renewal retries from the latest inventory.
	resourceSvc.syntheticBackend = &syntheticUpdateHookBackend{
		SyntheticDiscoveryConfigs: baseSyntheticBackend,
		hook: func() error {
			current, err := baseSyntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
			if err != nil {
				return err
			}
			current.Status.Synthetic.DiscoveryGroup = "group-three"
			_, err = baseSyntheticBackend.ConditionalUpdateSyntheticDiscoveryConfig(ctx, current)
			return err
		},
	}
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report})
	require.NoError(t, err)
	stored, err = baseSyntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "group-three", stored.ConfiguredDiscoveryGroup())
	require.Equal(t, uint64(42), stored.Status.DiscoveredResources)
}

func TestSyntheticPublicationSupportsCountFallbackWhenMergedRecordIsTooLarge(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.SyntheticName(serverID)

	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("group"))
	require.NoError(t, err)

	// The report fits with the current inventory and leaves room for count-form
	// inventory, but not for a later detailed inventory near its own budget.
	errorMessage := strings.Repeat("x", discoveryconfig.MaxSyntheticDiscoveryConfigSize-16*1024)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{
		State:        discoveryconfigpb.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR,
		ErrorMessage: &errorMessage,
	}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report})
	require.NoError(t, err)

	detailed := syntheticPublicationRequest("group")
	detailed.Synthetic.Matchers.Aws[0].Regions = []string{strings.Repeat("r", 32*1024)}
	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, detailed)
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)

	counts := &discoveryconfigpb.UpsertSyntheticDiscoveryConfigRequest{Synthetic: &discoveryconfigpb.SyntheticDiscoveryConfigStatus{
		DiscoveryGroup:    "group",
		MatchersTruncated: true,
		MatcherCounts:     discoveryconfigpb.StaticMatcherCounts_builder{Aws: 1}.Build(),
	}}
	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, counts)
	require.NoError(t, err)

	stored, err := resourceSvc.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.True(t, stored.Status.Synthetic.MatchersTruncated)
	require.Equal(t, uint32(1), stored.Status.Synthetic.MatcherCounts.AWS)
	require.Equal(t, errorMessage, *stored.Status.ErrorMessage, "inventory fallback must preserve report fields")
}

func TestSyntheticStatusRejectsReportThatConsumesInventoryHeadroom(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.SyntheticName(serverID)

	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("group"))
	require.NoError(t, err)

	errorMessage := strings.Repeat("x", discoveryconfig.MaxSyntheticDiscoveryConfigSize)
	report := discoveryconfigpb.DiscoveryConfigStatus_builder{ErrorMessage: &errorMessage}.Build()
	_, err = resourceSvc.UpdateDiscoveryConfigStatus(ownerCtx, &discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name, Status: report})
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)

	stored, err := resourceSvc.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)
	require.Nil(t, stored.Status.ErrorMessage, "rejected report must not replace stored status")
}

// TestSyntheticReadThenWriteContract covers get-then-mutate flows (web UI
// edits, tctl edit): a synthetic resource is readable through the explicit
// synthetic RPC, and every generic write against it must fail with the
// ownership story — not "discovery group is missing" from its stripped,
// empty-spec payload, and not NotFound.
func TestSyntheticReadThenWriteContract(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, localClient, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.SyntheticName(serverID)
	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("synthetic-group"))
	require.NoError(t, err)

	userCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbUpdate},
	}}}}, localClient)
	got, err := resourceSvc.GetSyntheticDiscoveryConfig(userCtx, &discoveryconfigpb.GetDiscoveryConfigRequest{Name: name})
	require.NoError(t, err)

	_, err = resourceSvc.UpdateDiscoveryConfig(userCtx, &discoveryconfigpb.UpdateDiscoveryConfigRequest{DiscoveryConfig: got})
	require.True(t, trace.IsAccessDenied(err), "got %v", err)
	require.Contains(t, err.Error(), "owner-managed")
	_, err = resourceSvc.UpsertDiscoveryConfig(userCtx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{DiscoveryConfig: got})
	require.True(t, trace.IsAccessDenied(err), "got %v", err)
	require.Contains(t, err.Error(), "owner-managed")
	_, err = resourceSvc.CreateDiscoveryConfig(userCtx, &discoveryconfigpb.CreateDiscoveryConfigRequest{DiscoveryConfig: got})
	require.True(t, trace.IsAccessDenied(err), "got %v", err)
	require.Contains(t, err.Error(), "owner-managed")
}

// TestSyntheticPublicationDoesNotResurrectDeletedStatus covers the retry-loop
// race where an update attempt observes an existing record (merging its
// status into the working copy), loses the CAS race, and the record is
// deleted before the next iteration: the create that follows must publish a
// fresh status, not the deleted record's.
func TestSyntheticPublicationDoesNotResurrectDeletedStatus(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)

	stale, err := discoveryconfig.NewSyntheticDiscoveryConfig(serverID, discoveryconfig.SyntheticStatus{
		DiscoveryGroup: "stale-group",
		Matchers:       &discoveryconfig.Spec{},
	})
	require.NoError(t, err)
	stale.Status.DiscoveredResources = 7
	stale.SetRevision("stale-revision")

	race := &syntheticDeletionRaceBackend{stale: stale}
	resourceSvc.syntheticBackend = race

	_, err = resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("fresh-group"))
	require.NoError(t, err)
	require.NotNil(t, race.created)
	require.Equal(t, "fresh-group", race.created.ConfiguredDiscoveryGroup())
	require.Zero(t, race.created.Status.DiscoveredResources,
		"a fresh publication must not resurrect status from a deleted record")
	require.Empty(t, race.created.Status.ServerStatus)
	require.Empty(t, race.created.GetRevision())
}

// syntheticDeletionRaceBackend scripts the deletion race: the first Get
// returns a pre-existing record, the update attempt against it loses the CAS
// race, and every later Get reports the record gone so the retry takes the
// create branch.
type syntheticDeletionRaceBackend struct {
	services.SyntheticDiscoveryConfigs
	stale   *discoveryconfig.DiscoveryConfig
	gets    int
	created *discoveryconfig.DiscoveryConfig
}

func (b *syntheticDeletionRaceBackend) GetSyntheticDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	b.gets++
	if b.gets == 1 {
		return b.stale, nil
	}
	return nil, trace.NotFound("synthetic discovery config %q not found", name)
}

func (b *syntheticDeletionRaceBackend) ConditionalUpdateSyntheticDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.CompareFailed("concurrent write")
}

func (b *syntheticDeletionRaceBackend) CreateSyntheticDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	b.created = dc.Clone()
	return dc, nil
}

func TestSyntheticGenericReadAndDeletion(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, localClient, resourceSvc := initSvc(t, clusterName)
	ownerCtx := authorizerForDiscoveryOwner(ctx, clusterName, serverID)
	name := discoveryconfig.SyntheticName(serverID)
	_, err := resourceSvc.UpsertSyntheticDiscoveryConfig(ownerCtx, syntheticPublicationRequest("synthetic-group"))
	require.NoError(t, err)

	userCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbRead, types.VerbList, types.VerbDelete},
	}}}}, localClient)
	// The legacy Get RPC is regular-only so pre-existing clients never
	// receive a synthetic resource they cannot decode; synthetic inventory is
	// served by the explicit synthetic RPC and current API clients combine
	// the two client-side.
	_, err = resourceSvc.GetDiscoveryConfig(userCtx, &discoveryconfigpb.GetDiscoveryConfigRequest{Name: name})
	require.True(t, trace.IsNotFound(err), "got %v", err)
	got, err := resourceSvc.GetSyntheticDiscoveryConfig(userCtx, &discoveryconfigpb.GetDiscoveryConfigRequest{Name: name})
	require.NoError(t, err)
	require.Equal(t, discoveryconfig.SubKindSynthetic, got.GetHeader().GetSubKind())

	_, err = resourceSvc.DeleteDiscoveryConfig(userCtx, &discoveryconfigpb.DeleteDiscoveryConfigRequest{Name: name})
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	// Bulk deletion clears the regular store but must not reach the isolated
	// synthetic record.
	doomed, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "regular-config"}, discoveryconfig.Spec{DiscoveryGroup: "regular-group"})
	require.NoError(t, err)
	_, err = resourceSvc.backend.CreateDiscoveryConfig(ctx, doomed)
	require.NoError(t, err)
	_, err = resourceSvc.DeleteAllDiscoveryConfigs(userCtx, &discoveryconfigpb.DeleteAllDiscoveryConfigsRequest{})
	require.NoError(t, err)
	_, err = resourceSvc.backend.GetDiscoveryConfig(ctx, doomed.GetName())
	require.True(t, trace.IsNotFound(err), "got %v", err)
	_, err = resourceSvc.syntheticBackend.GetSyntheticDiscoveryConfig(ctx, name)
	require.NoError(t, err)

	// A pre-existing regular config with the same name is served by the
	// legacy Get RPC without affecting the isolated synthetic record.
	regular, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: name}, discoveryconfig.Spec{DiscoveryGroup: "regular-group"})
	require.NoError(t, err)
	_, err = resourceSvc.backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	got, err = resourceSvc.GetDiscoveryConfig(userCtx, &discoveryconfigpb.GetDiscoveryConfigRequest{Name: name})
	require.NoError(t, err)
	require.Empty(t, got.GetHeader().GetSubKind())
	require.Equal(t, "regular-group", got.GetSpec().GetDiscoveryGroup())
	got, err = resourceSvc.GetSyntheticDiscoveryConfig(userCtx, &discoveryconfigpb.GetDiscoveryConfigRequest{Name: name})
	require.NoError(t, err)
	require.Equal(t, discoveryconfig.SubKindSynthetic, got.GetHeader().GetSubKind())
}

func TestRegularReservedAndLegacyNames(t *testing.T) {
	ctx, localClient, resourceSvc := initSvc(t, "test-cluster")
	writeCtx := authorizerForDummyUser(t, ctx, types.RoleSpecV6{Allow: types.RoleConditions{Rules: []types.Rule{{
		Resources: []string{types.KindDiscoveryConfig},
		Verbs:     []string{types.VerbCreate, types.VerbUpdate, types.VerbDelete},
	}}}}, localClient)

	reserved, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: discoveryconfig.SyntheticName(uuid.NewString())},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	// Reserved names that are not in use fail with BadParameter, not
	// AlreadyExists: the name can be neither read nor deleted, so claiming it
	// exists would send IaC reconcile loops chasing a phantom resource.
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, &discoveryconfigpb.CreateDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(reserved)})
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(reserved)})
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "can be updated but not recreated")
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, &discoveryconfigpb.UpdateDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(reserved)})
	require.True(t, trace.IsNotFound(err), "got %v", err)
	_, err = resourceSvc.DeleteDiscoveryConfig(writeCtx, &discoveryconfigpb.DeleteDiscoveryConfigRequest{Name: reserved.GetName()})
	require.True(t, trace.IsNotFound(err), "got %v", err)

	// A reserved-shaped regular config predating the reservation remains
	// updatable, but deletion is terminal. If it disappears after the upsert's
	// existence check, report the same reserved-name error as an initially
	// absent name rather than leaking the internal Update NotFound.
	_, err = resourceSvc.backend.CreateDiscoveryConfig(ctx, reserved)
	require.NoError(t, err)
	// An occupied reserved name answers create with AlreadyExists without
	// reaching the backend, where racing a concurrent deletion could recreate
	// the reserved name.
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, &discoveryconfigpb.CreateDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(reserved)})
	require.True(t, trace.IsAlreadyExists(err), "got %v", err)
	racingBackend := &regularUpdateHookBackend{
		DiscoveryConfigs: resourceSvc.backend,
	}
	racingBackend.beforeUpdate = func() error {
		return racingBackend.DiscoveryConfigs.DeleteDiscoveryConfig(ctx, reserved.GetName())
	}
	resourceSvc.backend = racingBackend
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(reserved)})
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Contains(t, err.Error(), "can be updated but not recreated")

	legacy, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "synthetic-aws-prod"},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	_, err = resourceSvc.CreateDiscoveryConfig(writeCtx, &discoveryconfigpb.CreateDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(legacy)})
	require.NoError(t, err)
	legacy.Spec.DiscoveryGroup = "updated"
	_, err = resourceSvc.UpdateDiscoveryConfig(writeCtx, &discoveryconfigpb.UpdateDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(legacy)})
	require.NoError(t, err)
	_, err = resourceSvc.UpsertDiscoveryConfig(writeCtx, &discoveryconfigpb.UpsertDiscoveryConfigRequest{DiscoveryConfig: convert.ToProto(legacy)})
	require.NoError(t, err)
}

func TestDeletedGrandfatheredConfigStatusReturnsNotFound(t *testing.T) {
	const clusterName = "test-cluster"
	serverID := uuid.NewString()
	ctx, _, resourceSvc := initSvc(t, clusterName)
	name := discoveryconfig.SyntheticName(uuid.NewString())

	regular, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: name},
		discoveryconfig.Spec{DiscoveryGroup: "group"},
	)
	require.NoError(t, err)
	_, err = resourceSvc.backend.CreateDiscoveryConfig(ctx, regular)
	require.NoError(t, err)
	require.NoError(t, resourceSvc.backend.DeleteDiscoveryConfig(ctx, name))

	_, err = resourceSvc.UpdateDiscoveryConfigStatus(
		authorizerForDiscoveryOwner(ctx, clusterName, serverID),
		&discoveryconfigpb.UpdateDiscoveryConfigStatusRequest{Name: name},
	)
	require.True(t, trace.IsNotFound(err), "got %v", err)
}

type regularUpdateHookBackend struct {
	services.DiscoveryConfigs
	beforeUpdate func() error
}

func (b *regularUpdateHookBackend) UpdateDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if b.beforeUpdate != nil {
		beforeUpdate := b.beforeUpdate
		b.beforeUpdate = nil
		if err := beforeUpdate(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return b.DiscoveryConfigs.UpdateDiscoveryConfig(ctx, dc)
}

func syntheticPublicationRequest(group string) *discoveryconfigpb.UpsertSyntheticDiscoveryConfigRequest {
	return &discoveryconfigpb.UpsertSyntheticDiscoveryConfigRequest{Synthetic: &discoveryconfigpb.SyntheticDiscoveryConfigStatus{
		DiscoveryGroup: group,
		Matchers: &discoveryconfigpb.DiscoveryConfigSpec{Aws: []*types.AWSMatcher{{
			Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
		}}},
	}}
}

type syntheticUpdateHookBackend struct {
	services.SyntheticDiscoveryConfigs
	hook func() error
}

func (b *syntheticUpdateHookBackend) ConditionalUpdateSyntheticDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	if b.hook != nil {
		hook := b.hook
		b.hook = nil
		if err := hook(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return b.SyntheticDiscoveryConfigs.ConditionalUpdateSyntheticDiscoveryConfig(ctx, dc)
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
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
	services.Presence
}

func initSvc(t *testing.T, clusterName string) (context.Context, localClient, *Service) {
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

	emitter := events.NewDiscardEmitter()

	resourceSvc, err := NewService(ServiceConfig{
		Backend:          localResourceService,
		SyntheticBackend: localResourceService,
		Authorizer:       authorizer,
		Emitter:          emitter,
		UsageReporter:    usagereporter.DiscardUsageReporter{},
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
