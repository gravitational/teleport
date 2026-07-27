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
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
)

func newTestTLSServer(t testing.TB) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
		ScopesFeatures: scopes.Features{
			Enabled:         true,
			AgentPinEnabled: true,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

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

	user, role, err := authtest.CreateUserAndRole(
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

	unprivilegedUser, unprivilegedRole, err := authtest.CreateUserAndRole(
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
			req: presencev1pb.GetRemoteClusterRequest_builder{
				Name: matchingRC.GetName(),
			}.Build(),
			assertError: require.NoError,
			want:        matchingRC.(*types.RemoteClusterV3),
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.GetRemoteClusterRequest_builder{
				Name: matchingRC.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "no permissions - unmatching rc",
			user: user.GetName(),
			req: presencev1pb.GetRemoteClusterRequest_builder{
				Name: notMatchingRC.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				// Opaque no permission presents as not found
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name: "validation - no name",
			user: user.GetName(),
			req: presencev1pb.GetRemoteClusterRequest_builder{
				Name: "",
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be specified")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "doesnt exist",
			user: user.GetName(),
			req: presencev1pb.GetRemoteClusterRequest_builder{
				Name: "non-existent",
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
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
		client, err := srv.NewClient(authtest.TestUser(user.GetName()))
		require.NoError(t, err)

		_, doesntExistError := client.PresenceServiceClient().GetRemoteCluster(ctx, presencev1pb.GetRemoteClusterRequest_builder{
			Name: "non-existent",
		}.Build())
		require.Error(t, doesntExistError)
		_, noPermissionsError := client.PresenceServiceClient().GetRemoteCluster(ctx, presencev1pb.GetRemoteClusterRequest_builder{
			Name: notMatchingRC.GetName(),
		}.Build())
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

	user, role, err := authtest.CreateUserAndRole(
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

	unprivilegedUser, unprivilegedRole, err := authtest.CreateUserAndRole(
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
			want: presencev1pb.ListRemoteClustersResponse_builder{
				RemoteClusters: []*types.RemoteClusterV3{
					matchingRC.(*types.RemoteClusterV3),
					matchingRC2.(*types.RemoteClusterV3),
				},
			}.Build(),
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
			client, err := srv.NewClient(authtest.TestUser(tt.user))
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

	user, _, err := authtest.CreateUserAndRole(
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

	unprivilegedUser, _, err := authtest.CreateUserAndRole(
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
			req: presencev1pb.DeleteRemoteClusterRequest_builder{
				Name: rc.GetName(),
			}.Build(),
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.DeleteRemoteClusterRequest_builder{
				Name: rc.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: user.GetName(),
			req: presencev1pb.DeleteRemoteClusterRequest_builder{
				Name: rc.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().DeleteRemoteCluster(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetRemoteCluster(ctx, tt.req.GetName())
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

	user, _, err := authtest.CreateUserAndRole(
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

	unprivilegedUser, _, err := authtest.CreateUserAndRole(
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
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
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
			}.Build(),

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
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
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
			}.Build(),

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
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
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
			}.Build(),

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
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
				RemoteCluster: &types.RemoteClusterV3{
					Metadata: types.Metadata{
						Name: rc.GetName(),
					},
				},
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "validation - nil rc",
			user: user.GetName(),
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
				RemoteCluster: nil,
				UpdateMask:    nil,
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "remote_cluster: must not be nil")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - no name",
			user: user.GetName(),
			req: presencev1pb.UpdateRemoteClusterRequest_builder{
				RemoteCluster: &types.RemoteClusterV3{
					Metadata: types.Metadata{
						Name: "",
					},
				},
				UpdateMask: nil,
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "remote_cluster.Metadata.Name: must be non-empty")
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
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

// TestListAuthServers is an integration test that uses a real gRPC
// client/server.
func TestListAuthServers(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"auth-server-lister",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindAuthServer},
				Verbs:     []string{types.VerbList, types.VerbRead},
			},
		})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	unprivilegedUser, unprivilegedRole, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unprivilegedRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindAuthServer},
			Verbs:     []string{types.VerbList},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, unprivilegedRole)
	require.NoError(t, err)

	// Create a few auth servers
	created := []*types.ServerV2{}
	for i := range 3 {
		server := &types.ServerV2{
			Kind:    types.KindAuthServer,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      fmt.Sprintf("auth-%d", i),
				Namespace: "default",
			},
			Spec: types.ServerSpecV2{
				Addr: fmt.Sprintf("127.0.0.1:%d", 3025+i),
			},
		}
		require.NoError(t, srv.Auth().UpsertAuthServer(ctx, server))
		created = append(created, server)
	}

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.ListAuthServersRequest
		assertError require.ErrorAssertionFunc
		want        []*types.ServerV2
	}{
		{
			name:        "success",
			user:        user.GetName(),
			req:         &presencev1pb.ListAuthServersRequest{},
			assertError: require.NoError,
			want:        created,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &presencev1pb.ListAuthServersRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.PresenceServiceClient().ListAuthServers(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				require.Empty(
					t, cmp.Diff(
						tt.want,
						res.GetServers(),
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					),
				)
			}
		})
	}

	t.Run("pagination", func(t *testing.T) {
		client, err := srv.NewClient(authtest.TestUser(user.GetName()))
		require.NoError(t, err)

		allGot := []*types.ServerV2{}
		pageToken := ""
		for i := range 3 {
			var got []types.Server
			got, pageToken, err = client.ListAuthServers(ctx, 1, pageToken)
			require.NoError(t, err)
			if i == 2 {
				require.Empty(t, pageToken)
			} else {
				require.NotEmpty(t, pageToken)
			}
			require.Len(t, got, 1)
			for _, item := range got {
				allGot = append(allGot, item.(*types.ServerV2))
			}
		}
		require.Len(t, allGot, 3)

		require.Empty(
			t, cmp.Diff(
				allGot,
				created,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
			),
		)
	})
}

// TestListProxyServers is an integration test that uses a real gRPC
// client/server.
func TestListProxyServers(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"proxy-server-lister",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindProxy},
				Verbs:     []string{types.VerbList, types.VerbRead},
			},
		})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	unprivilegedUser, unprivilegedRole, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unprivilegedRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindProxy},
			Verbs:     []string{types.VerbList},
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, unprivilegedRole)
	require.NoError(t, err)

	// Create a few proxy servers
	created := []*types.ServerV2{}
	for i := range 3 {
		server := &types.ServerV2{
			Kind:    types.KindProxy,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      fmt.Sprintf("proxy-%d", i),
				Namespace: "default",
			},
			Spec: types.ServerSpecV2{
				Addr: fmt.Sprintf("127.0.0.1:%d", 3080+i),
			},
		}
		_, err := srv.Auth().UpsertProxyServer(ctx, server)
		require.NoError(t, err)
		created = append(created, server)
	}

	tests := []struct {
		name        string
		user        string
		req         *presencev1pb.ListProxyServersRequest
		assertError require.ErrorAssertionFunc
		want        []*types.ServerV2
	}{
		{
			name:        "success",
			user:        user.GetName(),
			req:         &presencev1pb.ListProxyServersRequest{},
			assertError: require.NoError,
			want:        created,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req:  &presencev1pb.ListProxyServersRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.PresenceServiceClient().ListProxyServers(ctx, tt.req)
			tt.assertError(t, err)
			if tt.want != nil {
				require.Empty(
					t, cmp.Diff(
						tt.want,
						res.GetServers(),
						cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					),
				)
			}
		})
	}

	t.Run("pagination", func(t *testing.T) {
		client, err := srv.NewClient(authtest.TestUser(user.GetName()))
		require.NoError(t, err)

		allGot := []*types.ServerV2{}
		pageToken := ""
		for i := range 3 {
			var got []types.Server
			got, pageToken, err = client.ListProxyServers(ctx, 1, pageToken)
			require.NoError(t, err)
			if i == 2 {
				require.Empty(t, pageToken)
			} else {
				require.NotEmpty(t, pageToken)
			}
			require.Len(t, got, 1)
			for _, item := range got {
				allGot = append(allGot, item.(*types.ServerV2))
			}
		}
		require.Len(t, allGot, 3)

		require.Empty(
			t, cmp.Diff(
				allGot,
				created,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
			),
		)
	})
}

// TestListReverseTunnels is an integration test that uses a real gRPC
// client/server.
func TestListReverseTunnels(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, role, err := authtest.CreateUserAndRole(
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

	unprivilegedUser, unprivilegedRole, err := authtest.CreateUserAndRole(
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
			want: presencev1pb.ListReverseTunnelsResponse_builder{
				ReverseTunnels: created,
			}.Build(),
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
			client, err := srv.NewClient(authtest.TestUser(tt.user))
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
		client, err := srv.NewClient(authtest.TestUser(user.GetName()))
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

	user, _, err := authtest.CreateUserAndRole(
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
	unprivilegedUser, _, err := authtest.CreateUserAndRole(
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
			req: presencev1pb.DeleteReverseTunnelRequest_builder{
				Name: rt.GetName(),
			}.Build(),
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.DeleteReverseTunnelRequest_builder{
				Name: rt.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: user.GetName(),
			req: presencev1pb.DeleteReverseTunnelRequest_builder{
				Name: rt.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().DeleteReverseTunnel(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				_, err := srv.Auth().GetReverseTunnel(ctx, tt.req.GetName())
				require.True(t, trace.IsNotFound(err), "rt should be deleted")
			}
		})
	}
}

// TestDeleteProxyServer is an integration test that uses a real gRPC client/server.
func TestDeleteProxyServer(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := t.Context()

	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"proxy-deleter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindProxy},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	unprivilegedUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"proxy-no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	proxy, err := types.NewServer("proxy-1", types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertProxyServer(ctx, proxy)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		user                  string
		req                   *presencev1pb.DeleteProxyServerRequest
		assertError           require.ErrorAssertionFunc
		checkResourcesDeleted bool
	}{
		{
			name: "success",
			user: user.GetName(),
			req: presencev1pb.DeleteProxyServerRequest_builder{
				Name: proxy.GetName(),
			}.Build(),
			assertError:           require.NoError,
			checkResourcesDeleted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.DeleteProxyServerRequest_builder{
				Name: proxy.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "non existent",
			user: user.GetName(),
			req: presencev1pb.DeleteProxyServerRequest_builder{
				Name: proxy.GetName(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "error should be not found")
			},
		},
		{
			name: "missing name",
			user: user.GetName(),
			req:  &presencev1pb.DeleteProxyServerRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().DeleteProxyServer(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourcesDeleted {
				proxies, err := stream.Collect(clientutils.Resources(ctx, srv.Auth().ListProxyServers))
				require.NoError(t, err)
				for _, p := range proxies {
					require.NotEqual(t, tt.req.GetName(), p.GetName(), "proxy should be deleted")
				}
			}
		})
	}
}

// TestUpsertProxyServer is an integration test that uses a real gRPC client/server.
func TestUpsertProxyServer(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := t.Context()

	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"proxy-upserter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindProxy},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	unprivilegedUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"proxy-upsert-no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	proxy, err := types.NewServer("proxy-1", types.KindProxy, types.ServerSpecV2{
		Addr:     "127.0.0.1:2023",
		Hostname: "proxy.llama",
	})
	require.NoError(t, err)

	tests := []struct {
		name                  string
		user                  string
		req                   *presencev1pb.UpsertProxyServerRequest
		assertError           require.ErrorAssertionFunc
		checkResourceUpserted bool
	}{
		{
			name: "success",
			user: user.GetName(),
			req: presencev1pb.UpsertProxyServerRequest_builder{
				Server: proxy.(*types.ServerV2),
			}.Build(),
			assertError:           require.NoError,
			checkResourceUpserted: true,
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.UpsertProxyServerRequest_builder{
				Server: proxy.(*types.ServerV2),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "missing server",
			user: user.GetName(),
			req:  &presencev1pb.UpsertProxyServerRequest{},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation failure - blank name",
			user: user.GetName(),
			req: presencev1pb.UpsertProxyServerRequest_builder{
				Server: &types.ServerV2{
					Kind:    types.KindProxy,
					Version: types.V2,
					Metadata: types.Metadata{
						Name: "",
					},
				},
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			_, err = client.PresenceServiceClient().UpsertProxyServer(ctx, tt.req)
			tt.assertError(t, err)
			if tt.checkResourceUpserted {
				proxies, err := stream.Collect(clientutils.Resources(ctx, srv.Auth().ListProxyServers))
				require.NoError(t, err)
				var found bool
				for _, p := range proxies {
					if p.GetName() == tt.req.GetServer().GetName() {
						found = true
						break
					}
				}
				require.True(t, found, "proxy should have been upserted")
			}
		})
	}
}

func TestUpsertReverseTunnel(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user, _, err := authtest.CreateUserAndRole(
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
	unprivilegedUser, _, err := authtest.CreateUserAndRole(
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
			req: presencev1pb.UpsertReverseTunnelRequest_builder{
				ReverseTunnel: rt.(*types.ReverseTunnelV2),
			}.Build(),
			assertError: require.NoError,
			want:        rt.(*types.ReverseTunnelV2),
		},
		{
			name: "no permissions",
			user: unprivilegedUser.GetName(),
			req: presencev1pb.UpsertReverseTunnelRequest_builder{
				ReverseTunnel: rt.(*types.ReverseTunnelV2),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "error should be access denied")
			},
		},
		{
			name: "no value",
			user: user.GetName(),
			req: presencev1pb.UpsertReverseTunnelRequest_builder{
				ReverseTunnel: nil,
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "error should be bad parameter")
			},
		},
		{
			name: "validation - invalid",
			user: user.GetName(),
			req: presencev1pb.UpsertReverseTunnelRequest_builder{
				ReverseTunnel: invalid.(*types.ReverseTunnelV2),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "failed to parse")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
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

// TestDeleteAppServer is an integration test that uses a real gRPC
// client/server to exercise deleting scoped and unscoped application servers
// through the presence service with authorized and unauthorized callers.
func TestDeleteAppServer(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := t.Context()
	const (
		parentScope     = "/aa"
		scope           = "/aa/aa"
		orthogonalScope = "/aa/bb"
	)

	deleteUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"app-server-deleter",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindAppServer},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	unprivilegedUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"app-server-no-perms",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)

	createScopedRole := func(name string, verbs []string) *scopedaccessv1.ScopedRole {
		scopedRole, err := srv.Auth().ScopedAccess().CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:    scopedaccess.KindScopedRole,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: name,
				}.Build(),
				Scope: parentScope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{scope, orthogonalScope},
					Rules: []*scopedaccessv1.ScopedRule{
						scopedaccessv1.ScopedRule_builder{
							Resources: []string{types.KindAppServer},
							Verbs:     verbs,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		return scopedRole.GetRole()
	}

	scopedDeleteRole := createScopedRole("app-server-delete-role", []string{types.VerbDelete})
	scopedReadRole := createScopedRole("app-server-read-role", []string{types.VerbRead})

	createAssignment := func(role *scopedaccessv1.ScopedRole, username, assignedScope string) *scopedaccessv1.ScopedRoleAssignment {
		sra, err := srv.Auth().ScopedAccess().CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				SubKind: scopedaccess.SubKindDynamic,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Scope: assignedScope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: username,
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  scopes.QualifiedName{Scope: role.GetScope(), Name: role.GetMetadata().GetName()}.String(),
							Scope: assignedScope,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		return sra.GetAssignment()
	}

	waitForSRACache(t, srv,
		createAssignment(scopedDeleteRole, deleteUser.GetName(), scope),
		createAssignment(scopedReadRole, unprivilegedUser.GetName(), scope),
	)

	// newAppServer registers a fresh app server in the given scope. Each test
	// case gets its own server since successful deletions are destructive.
	newAppServer := func(t *testing.T, scope string) types.AppServer {
		t.Helper()
		name := "app-" + uuid.NewString()
		spec := types.AppSpecV3{URI: "http://localhost:8080"}
		if scope != "" {
			// Scoped apps must register with their derived public address.
			spec.PublicAddr = scopedapp.ScopedAppPublicAddr(scope, name, "proxy.example.com")
		}
		app, err := types.NewAppV3(types.Metadata{Name: name}, spec, scope)
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(app, "localhost", uuid.NewString())
		require.NoError(t, err)
		_, err = srv.Auth().UpsertApplicationServer(ctx, server)
		require.NoError(t, err)
		return server
	}

	tests := []struct {
		name        string
		identity    authtest.TestIdentity
		targetScope string
		// req overrides the request derived from a freshly created target.
		req          *presencev1pb.DeleteAppServerRequest
		assertError  require.ErrorAssertionFunc
		checkDeleted bool
	}{
		{
			name:         "unscoped user with delete rule deletes unscoped server",
			identity:     authtest.TestUser(deleteUser.GetName()),
			targetScope:  "",
			assertError:  require.NoError,
			checkDeleted: true,
		},
		{
			name:         "unscoped user with delete rule deletes scoped server",
			identity:     authtest.TestUser(deleteUser.GetName()),
			targetScope:  scope,
			assertError:  require.NoError,
			checkDeleted: true,
		},
		{
			name:        "unscoped user without delete rule cannot delete unscoped server",
			identity:    authtest.TestUser(unprivilegedUser.GetName()),
			targetScope: "",
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
			},
		},
		{
			name:        "unscoped user without delete rule cannot delete scoped server",
			identity:    authtest.TestUser(unprivilegedUser.GetName()),
			targetScope: scope,
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
			},
		},
		{
			name:         "scoped user with delete rule deletes server in its scope",
			identity:     authtest.TestScopedUser(deleteUser.GetName(), scope),
			targetScope:  scope,
			assertError:  require.NoError,
			checkDeleted: true,
		},
		{
			name:        "scoped user with delete rule cannot delete server in orthogonal scope",
			identity:    authtest.TestScopedUser(deleteUser.GetName(), scope),
			targetScope: orthogonalScope,
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
			},
		},
		{
			name:        "scoped user with delete rule cannot delete unscoped server",
			identity:    authtest.TestScopedUser(deleteUser.GetName(), scope),
			targetScope: "",
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
			},
		},
		{
			name:        "scoped user with read-only rule cannot delete server in its scope",
			identity:    authtest.TestScopedUser(unprivilegedUser.GetName(), scope),
			targetScope: scope,
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
			},
		},
		{
			name:     "missing host_id",
			identity: authtest.TestUser(deleteUser.GetName()),
			req: presencev1pb.DeleteAppServerRequest_builder{
				Name: "some-app",
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
			},
		},
		{
			name:     "missing name",
			identity: authtest.TestUser(deleteUser.GetName()),
			req: presencev1pb.DeleteAppServerRequest_builder{
				HostId: uuid.NewString(),
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
			},
		},
		{
			name:     "non existent server",
			identity: authtest.TestUser(deleteUser.GetName()),
			req: presencev1pb.DeleteAppServerRequest_builder{
				HostId: uuid.NewString(),
				Name:   "does-not-exist",
			}.Build(),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(tt.identity)
			require.NoError(t, err)
			t.Cleanup(func() { client.Close() })

			req := tt.req
			var target types.AppServer
			if req == nil {
				target = newAppServer(t, tt.targetScope)
				req = presencev1pb.DeleteAppServerRequest_builder{
					HostId: target.GetHostID(),
					Name:   target.GetName(),
					Scope:  target.GetScope(),
				}.Build()
			}

			_, err = client.PresenceServiceClient().DeleteAppServer(t.Context(), req)
			tt.assertError(t, err)

			if tt.checkDeleted {
				servers, err := srv.Auth().GetApplicationServers(t.Context(), apidefaults.Namespace)
				require.NoError(t, err)
				for _, s := range servers {
					require.NotEqual(t, target.GetHostID(), s.GetHostID(), "app server should be deleted")
				}
			}
		})
	}
}

func waitForSRACache(t *testing.T, srv *authtest.TLSServer, sras ...*scopedaccessv1.ScopedRoleAssignment) {
	t.Helper()
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for _, sra := range sras {
			_, err := srv.Auth().ScopedAccessCache.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
				Name:    sra.GetMetadata().GetName(),
				SubKind: sra.GetSubKind(),
				Scope:   sra.GetScope(),
			}.Build())
			require.NoError(t, err)
		}
	}, 10*time.Second, 100*time.Millisecond)
}
