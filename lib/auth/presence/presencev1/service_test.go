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

package presencev1_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
)

func newTestTLSServer(t testing.TB) *auth.TestTLSServer {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestGetRemoteCluster is an integration test that uses a real gRPC
// client/server.
func TestGetRemoteCluster(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rc-getter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindRemoteCluster},
				Verbs:     []string{types.VerbRead},
			},
		})
	require.NoError(t, err)
	err = role.SetLabelMatchers(types.Allow, types.KindRemoteCluster, types.LabelMatchers{
		Labels: map[string]utils.Strings{
			"label": {"foo"},
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	unprivilegedUser, unprivilegedRole, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unprivilegedRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindRemoteCluster},
			Verbs:     []string{types.VerbRead},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, unprivilegedRole)
	require.NoError(t, err)

	matchingRC, err := types.NewRemoteCluster("matching")
	require.NoError(t, err)
	md := matchingRC.GetMetadata()
	md.Labels = map[string]string{"label": "foo"}
	matchingRC.SetMetadata(md)
	matchingRC, err = srv.Auth().CreateRemoteCluster(ctx, matchingRC)
	require.NoError(t, err)

	notMatchingRC, err := types.NewRemoteCluster("not-matching")
	require.NoError(t, err)
	md = notMatchingRC.GetMetadata()
	md.Labels = map[string]string{"label": "bar"}
	notMatchingRC.SetMetadata(md)
	notMatchingRC, err = srv.Auth().CreateRemoteCluster(ctx, notMatchingRC)
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.GetRemoteClusterRequest
		assertError require.ErrorAssertionFunc
		want        *types.RemoteClusterV3
	}{
		{
			name: "success",
			user: user.GetName(),
			req: &presencev1pb.GetRemoteClusterRequest{
				Name: matchingRC.GetName(),
			},
			assertError: require.NoError,
			want:        matchingRC.(*types.RemoteClusterV3),
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &presencev1pb.GetRemoteClusterRequest{
				Name: matchingRC.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "no permissions - unmatching rc",
			user: user.GetName(),
			req: &presencev1pb.GetRemoteClusterRequest{
				Name: notMatchingRC.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				// Opaque no permission presents as not found
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name: "validation - no name",
			user: user.GetName(),
			req: &presencev1pb.GetRemoteClusterRequest{
				Name: "",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be specified")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "doesnt exist",
			user: user.GetName(),
			req: &presencev1pb.GetRemoteClusterRequest{
				Name: "non-existent",
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			rc, err := client.PresenceServiceClient().GetRemoteCluster(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned remote cluster matches
				require.Empty(t, cmp.Diff(tt.want, rc, protocmp.Transform()))
			}
		})
	}

	t.Run("doesnt exist and no permissions errors match", func(t *testing.T) {
		client, err := srv.NewClient(auth.TestUser(user.GetName()))
		require.NoError(t, err)

		_, doesntExistError := client.PresenceServiceClient().GetRemoteCluster(ctx, &presencev1pb.GetRemoteClusterRequest{
			Name: "non-existent",
		})
		require.Error(t, doesntExistError)
		_, noPermissionsError := client.PresenceServiceClient().GetRemoteCluster(ctx, &presencev1pb.GetRemoteClusterRequest{
			Name: notMatchingRC.GetName(),
		})
		require.Error(t, noPermissionsError)

		require.Equal(t, doesntExistError.Error(), noPermissionsError.Error(),
			"the error message returned when the rc doesn't exist or when the user has no permission to see it should be indistinguishable")
		require.Equal(t, trail.ToGRPC(doesntExistError), trail.ToGRPC(noPermissionsError),
			"the gRPC error returned when the rc doesn't exist or when the user has no permission to see it should be indistinguishable")
	})
}

// TestListRemoteClusters is an integration test that uses a real gRPC
// client/server.
func TestListRemoteClusters(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rc-getter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindRemoteCluster},
				Verbs:     []string{types.VerbList},
			},
		})
	require.NoError(t, err)
	err = role.SetLabelMatchers(types.Allow, types.KindRemoteCluster, types.LabelMatchers{
		Labels: map[string]utils.Strings{
			"label": {"foo"},
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	unprivilegedUser, unprivilegedRole, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unprivilegedRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindRemoteCluster},
			Verbs:     []string{types.VerbList},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, unprivilegedRole)
	require.NoError(t, err)

	matchingRC, err := types.NewRemoteCluster("matching")
	require.NoError(t, err)
	md := matchingRC.GetMetadata()
	md.Labels = map[string]string{"label": "foo"}
	matchingRC.SetMetadata(md)
	matchingRC, err = srv.Auth().CreateRemoteCluster(ctx, matchingRC)
	require.NoError(t, err)

	matchingRC2, err := types.NewRemoteCluster("matching-2")
	require.NoError(t, err)
	md = matchingRC2.GetMetadata()
	md.Labels = map[string]string{"label": "foo"}
	matchingRC2.SetMetadata(md)
	matchingRC2, err = srv.Auth().CreateRemoteCluster(ctx, matchingRC2)
	require.NoError(t, err)

	notMatchingRC, err := types.NewRemoteCluster("not-matching")
	require.NoError(t, err)
	md = notMatchingRC.GetMetadata()
	md.Labels = map[string]string{"label": "bar"}
	notMatchingRC.SetMetadata(md)
	_, err = srv.Auth().CreateRemoteCluster(ctx, notMatchingRC)
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.ListRemoteClustersRequest
		assertError require.ErrorAssertionFunc
		want        *presencev1pb.ListRemoteClustersResponse
	}{
		{
			name:        "success",
			user:        user.GetName(),
			req:         &presencev1pb.ListRemoteClustersRequest{},
			assertError: require.NoError,
			want: &presencev1pb.ListRemoteClustersResponse{
				RemoteClusters: []*types.RemoteClusterV3{
					matchingRC.(*types.RemoteClusterV3),
					matchingRC2.(*types.RemoteClusterV3),
				},
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &presencev1pb.ListRemoteClustersRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.PresenceServiceClient().ListRemoteClusters(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned data matches
				require.Empty(
					t, cmp.Diff(
						tt.want,
						res,
						protocmp.Transform(),
						protocmp.SortRepeatedFields(&presencev1pb.ListRemoteClustersResponse{}, "remote_clusters"),
					),
				)
			}
		})
	}
}

// TestDeleteRemoteCluster is an integration test that uses a real gRPC client/server.
func TestDeleteRemoteCluster(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rc-deleter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindRemoteCluster},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)

	unprivilegedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	rc, err := types.NewRemoteCluster("matching")
	require.NoError(t, err)
	rc, err = srv.Auth().CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		user                  string
		req                   *presencev1pb.DeleteRemoteClusterRequest
		assertError           require.ErrorAssertionFunc
		checkResourcesDeleted bool
	}{
		{
			name: "success",
			user: user.GetName(),
			req: &presencev1pb.DeleteRemoteClusterRequest{
				Name: rc.GetName(),
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &presencev1pb.DeleteRemoteClusterRequest{
				Name: rc.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: user.GetName(),
			req: &presencev1pb.DeleteRemoteClusterRequest{
				Name: rc.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().DeleteRemoteCluster(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetRemoteCluster(ctx, tt.req.Name)
				require.True(t, trace.IsNotFound(err), "rc should be deleted")
			}
		})
	}
}

// TestUpdateRemoteCluster is an integration test that uses a real gRPC client/server.
func TestUpdateRemoteCluster(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rc-updater",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindRemoteCluster},
				Verbs:     []string{types.VerbUpdate},
			},
		})
	require.NoError(t, err)

	unprivilegedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	// Create pre-existing remote cluster so we can check you can update
	// an existing remote cluster.
	rc, err := types.NewRemoteCluster("rc")
	require.NoError(t, err)
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	rc, err = srv.Auth().CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	patchRC, err := types.NewRemoteCluster("patch")
	require.NoError(t, err)
	patchRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	patchRC, err = srv.Auth().CreateRemoteCluster(ctx, patchRC)
	require.NoError(t, err)

	partialPatchRC, err := types.NewRemoteCluster("partial-patch")
	require.NoError(t, err)
	partialPatchRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	partialPatchRC, err = srv.Auth().CreateRemoteCluster(ctx, partialPatchRC)
	require.NoError(t, err)

	expire := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		user string
		req  *presencev1pb.UpdateRemoteClusterRequest

		assertError require.ErrorAssertionFunc
		want        *types.RemoteClusterV3
	}{
		{
			name: "success",
			user: user.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: &types.RemoteClusterV3{
					Kind:    types.KindRemoteCluster,
					Version: types.V3,
					Metadata: types.Metadata{
						Name: rc.GetName(),
						Labels: map[string]string{
							"foo": "bar",
						},
						Revision: rc.GetRevision(),
					},
					Status: types.RemoteClusterStatusV3{
						Connection: teleport.RemoteClusterStatusOnline,
					},
				},
				UpdateMask: nil,
			},

			assertError: require.NoError,
			want: &types.RemoteClusterV3{
				Kind:    types.KindRemoteCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: rc.GetName(),
					Labels: map[string]string{
						"foo": "bar",
					},
					Namespace: rc.GetMetadata().Namespace,
				},
				Status: types.RemoteClusterStatusV3{
					Connection: teleport.RemoteClusterStatusOnline,
				},
			},
		},
		{
			name: "patch success",
			user: user.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: &types.RemoteClusterV3{
					Kind:    types.KindRemoteCluster,
					Version: types.V3,
					Metadata: types.Metadata{
						Name: patchRC.GetName(),
						Labels: map[string]string{
							"foo": "bar",
						},
						Expires:     &expire,
						Description: "patched",
					},
					Status: types.RemoteClusterStatusV3{
						Connection:    teleport.RemoteClusterStatusOnline,
						LastHeartbeat: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"Metadata.Labels",
						"Metadata.Expires",
						"Metadata.Description",
						"Status.Connection",
						"Status.LastHeartbeat",
					},
				},
			},

			assertError: require.NoError,
			want: &types.RemoteClusterV3{
				Kind:    types.KindRemoteCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: patchRC.GetName(),
					Labels: map[string]string{
						"foo": "bar",
					},
					Expires:     &expire,
					Description: "patched",
					Namespace:   "default",
				},
				Status: types.RemoteClusterStatusV3{
					Connection:    teleport.RemoteClusterStatusOnline,
					LastHeartbeat: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "partial patch success",
			user: user.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: &types.RemoteClusterV3{
					Kind:    types.KindRemoteCluster,
					Version: types.V3,
					Metadata: types.Metadata{
						Name: partialPatchRC.GetName(),
						Labels: map[string]string{
							"foo": "bar",
						},
						Expires:     &expire,
						Description: "patched",
					},
					Status: types.RemoteClusterStatusV3{
						Connection:    teleport.RemoteClusterStatusOnline,
						LastHeartbeat: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"Status.LastHeartbeat",
					},
				},
			},

			assertError: require.NoError,
			want: &types.RemoteClusterV3{
				Kind:    types.KindRemoteCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      partialPatchRC.GetName(),
					Namespace: "default",
				},
				Status: types.RemoteClusterStatusV3{
					Connection:    teleport.RemoteClusterStatusOffline,
					LastHeartbeat: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: &types.RemoteClusterV3{
					Metadata: types.Metadata{
						Name: rc.GetName(),
					},
				},
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - nil rc",
			user: user.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: nil,
				UpdateMask:    nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "remote_cluster: must not be nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: user.GetName(),
			req: &presencev1pb.UpdateRemoteClusterRequest{
				RemoteCluster: &types.RemoteClusterV3{
					Metadata: types.Metadata{
						Name: "",
					},
				},
				UpdateMask: nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "remote_cluster.Metadata.Name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			got, err := client.PresenceServiceClient().UpdateRemoteCluster(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned rc matches
				require.Empty(
					t,
					cmp.Diff(
						tt.want,
						got,
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					),
				)
			}
		})
	}
}

// TestListReverseTunnels is an integration test that uses a real gRPC
// client/server.
func TestListReverseTunnels(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rc-getter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindReverseTunnel},
				Verbs:     []string{types.VerbList, types.VerbRead},
			},
		})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	unprivilegedUser, unprivilegedRole, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unprivilegedRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindReverseTunnel},
			Verbs:     []string{types.VerbList},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, unprivilegedRole)
	require.NoError(t, err)

	// Create a few reverse tunnels
	created := []*types.ReverseTunnelV2{}
	for i := range 10 {
		rc, err := types.NewReverseTunnel(fmt.Sprintf("rt-%d", i), []string{"example.com:443"})
		require.NoError(t, err)
		_, err = srv.Auth().Services.UpsertReverseTunnel(ctx, rc)
		require.NoError(t, err)
		created = append(created, rc.(*types.ReverseTunnelV2))
	}

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.ListReverseTunnelsRequest
		assertError require.ErrorAssertionFunc
		want        *presencev1pb.ListReverseTunnelsResponse
	}{
		{
			name:        "success",
			user:        user.GetName(),
			req:         &presencev1pb.ListReverseTunnelsRequest{},
			assertError: require.NoError,
			want: &presencev1pb.ListReverseTunnelsResponse{
				ReverseTunnels: created,
			},
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &presencev1pb.ListReverseTunnelsRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.PresenceServiceClient().ListReverseTunnels(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned data matches
				require.Empty(
					t, cmp.Diff(
						tt.want,
						res,
						protocmp.Transform(),
						protocmp.SortRepeatedFields(&presencev1pb.ListReverseTunnelsResponse{}, "reverse_tunnels"),
					),
				)
			}
		})
	}

	t.Run("pagination", func(t *testing.T) {
		client, err := srv.NewClient(auth.TestUser(user.GetName()))
		require.NoError(t, err)

		allGot := []*types.ReverseTunnelV2{}
		pageToken := ""
		for i := range 10 {
			var got []types.ReverseTunnel
			got, pageToken, err = client.ListReverseTunnels(ctx, 1, pageToken)
			require.NoError(t, err)
			if i == 9 {
				// For the final page, we should not get a page token
				require.Empty(t, pageToken)
			} else {
				require.NotEmpty(t, pageToken)
			}
			require.Len(t, got, 1)
			for _, item := range got {
				allGot = append(allGot, item.(*types.ReverseTunnelV2))
			}
		}
		require.Len(t, allGot, 10)

		// Check that the returned data matches
		require.Empty(
			t, cmp.Diff(
				allGot,
				created),
		)
	})
}

// TestDeleteReverseTunnel is an integration test that uses a real gRPC client/server.
func TestDeleteReverseTunnel(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rt-deleter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindReverseTunnel},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	unprivilegedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	rt, err := types.NewReverseTunnel("example.com", []string{"example.com:443"})
	require.NoError(t, err)
	rt, err = srv.Auth().UpsertReverseTunnel(ctx, rt)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		user                  string
		req                   *presencev1pb.DeleteReverseTunnelRequest
		assertError           require.ErrorAssertionFunc
		checkResourcesDeleted bool
	}{
		{
			name: "success",
			user: user.GetName(),
			req: &presencev1pb.DeleteReverseTunnelRequest{
				Name: rt.GetName(),
			},
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &presencev1pb.DeleteReverseTunnelRequest{
				Name: rt.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: user.GetName(),
			req: &presencev1pb.DeleteReverseTunnelRequest{
				Name: rt.GetName(),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().DeleteReverseTunnel(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetReverseTunnel(ctx, tt.req.Name)
				require.True(t, trace.IsNotFound(err), "rt should be deleted")
			}
		})
	}
}

func TestUpsertReverseTunnel(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"rt-upserter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindReverseTunnel},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	unprivilegedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	rt, err := types.NewReverseTunnel("example.com", []string{"example.com:443"})
	require.NoError(t, err)

	invalid, err := types.NewReverseTunnel("example.com", []string{"!!://///example.com:44/./3!!!"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.UpsertReverseTunnelRequest
		assertError require.ErrorAssertionFunc
		want        *types.ReverseTunnelV2
	}{
		{
			name: "success",
			user: user.GetName(),
			req: &presencev1pb.UpsertReverseTunnelRequest{
				ReverseTunnel: rt.(*types.ReverseTunnelV2),
			},
			assertError: require.NoError,
			want:        rt.(*types.ReverseTunnelV2),
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: &presencev1pb.UpsertReverseTunnelRequest{
				ReverseTunnel: rt.(*types.ReverseTunnelV2),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "no value",
			user: user.GetName(),
			req: &presencev1pb.UpsertReverseTunnelRequest{
				ReverseTunnel: nil,
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - invalid",
			user: user.GetName(),
			req: &presencev1pb.UpsertReverseTunnelRequest{
				ReverseTunnel: invalid.(*types.ReverseTunnelV2),
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "failed to parse")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			got, err := client.PresenceServiceClient().UpsertReverseTunnel(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				// Check that the returned rt matches
				require.Empty(
					t,
					cmp.Diff(
						tt.want,
						got,
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					),
				)
			}
		})
	}

}
