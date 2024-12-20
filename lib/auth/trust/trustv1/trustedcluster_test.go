/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package trustv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

// TestCloudProhibited verifies that Trusted Clusters cannot be created or updated
// in a Cloud hosted environment.
// Tests cannot be run in parallel because it relies on environment variables.
func TestCloudProhibited(t *testing.T) {
	ctx := context.Background()
	p := newTestPack(t)

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: &fakeAuthorizer{},
		AuthServer: &fakeAuthServer{},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{Cloud: true},
	})

	tc, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{
		RoleMap: []types.RoleMapping{
			{Remote: teleport.PresetAccessRoleName, Local: []string{teleport.PresetAccessRoleName}},
		},
	})
	require.NoError(t, err, "creating trusted cluster resource")

	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	require.True(t, ok)

	t.Run("Cloud prohibits being a leaf cluster (UpsertTrustedCluster)", func(t *testing.T) {
		_, err = service.UpsertTrustedCluster(ctx, &trustpb.UpsertTrustedClusterRequest{
			TrustedCluster: trustedClusterV2,
		})
		require.True(t, trace.IsNotImplemented(err), "UpsertTrustedClusterV2 returned an unexpected error, got = %v (%T), want trace.NotImplementedError", err, err)
	})

	t.Run("Cloud prohibits being a leaf cluster (CreateTrustedCluster)", func(t *testing.T) {
		_, err = service.CreateTrustedCluster(ctx, &trustpb.CreateTrustedClusterRequest{
			TrustedCluster: trustedClusterV2,
		})
		require.True(t, trace.IsNotImplemented(err), "CreateTrustedCluster returned an unexpected error, got = %v (%T), want trace.NotImplementedError", err, err)
	})

	t.Run("Cloud prohibits being a leaf cluster (UpdateTrustedCluster)", func(t *testing.T) {
		_, err = service.UpdateTrustedCluster(ctx, &trustpb.UpdateTrustedClusterRequest{
			TrustedCluster: trustedClusterV2,
		})
		require.True(t, trace.IsNotImplemented(err), "UpdateTrustedCluster returned an unexpected error, got = %v (%T), want trace.NotImplementedError", err, err)
	})
}

func TestTrustedClusterRBAC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newTestPack(t)

	tc, err := types.NewTrustedCluster("test", types.TrustedClusterSpecV2{
		RoleMap: []types.RoleMapping{
			{Remote: teleport.PresetAccessRoleName, Local: []string{teleport.PresetAccessRoleName}},
		},
	})
	require.NoError(t, err, "creating trusted cluster resource")

	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	require.True(t, ok)

	tests := []struct {
		desc         string
		f            func(t *testing.T, service *Service)
		authorizer   fakeAuthorizer
		expectChecks []check
	}{
		{
			desc: "upsert no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertTrustedCluster(ctx, &trustpb.UpsertTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
		{
			desc: "upsert no create access",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertTrustedCluster(ctx, &trustpb.UpsertTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindTrustedCluster, types.VerbCreate}: false,
						{types.KindTrustedCluster, types.VerbUpdate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
		{
			desc: "upsert no update access",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertTrustedCluster(ctx, &trustpb.UpsertTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindTrustedCluster, types.VerbCreate}: true,
						{types.KindTrustedCluster, types.VerbUpdate}: false,
					},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
		{
			desc: "upsert ok",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertTrustedCluster(ctx, &trustpb.UpsertTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindTrustedCluster, types.VerbCreate}: true,
						{types.KindTrustedCluster, types.VerbUpdate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
		{
			desc: "create no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.CreateTrustedCluster(ctx, &trustpb.CreateTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
			},
		},
		{
			desc: "create ok",
			f: func(t *testing.T, service *Service) {
				_, err := service.CreateTrustedCluster(ctx, &trustpb.CreateTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindTrustedCluster, types.VerbCreate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbCreate},
			},
		},
		{
			desc: "update no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpdateTrustedCluster(ctx, &trustpb.UpdateTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
		{
			desc: "update ok",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpdateTrustedCluster(ctx, &trustpb.UpdateTrustedClusterRequest{
					TrustedCluster: trustedClusterV2,
				})
				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindTrustedCluster, types.VerbUpdate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindTrustedCluster, types.VerbUpdate},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			trust := local.NewCAService(p.mem)
			cfg := &ServiceConfig{
				Cache:      trust,
				Backend:    trust,
				Authorizer: &test.authorizer,
				AuthServer: &fakeAuthServer{},
			}

			service, err := NewService(cfg)
			require.NoError(t, err)
			test.f(t, service)
			require.ElementsMatch(t, test.expectChecks, test.authorizer.checker.checks)
		})
	}
}
