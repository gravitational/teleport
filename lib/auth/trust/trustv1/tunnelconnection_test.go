/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestUpsertTunnelConnection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	newConn := func(t *testing.T) *types.TunnelConnectionV2 {
		t.Helper()
		conn, err := types.NewTunnelConnection("conn-1", types.TunnelConnectionSpecV2{
			ClusterName:   "leaf.example.com",
			ProxyName:     "proxy-1",
			LastHeartbeat: time.Now().UTC(),
			Type:          types.ProxyTunnel,
		})
		require.NoError(t, err)
		return conn.(*types.TunnelConnectionV2)
	}

	tests := []struct {
		name      string
		req       func(t *testing.T) *trustpb.UpsertTunnelConnectionRequest
		allow     map[check]bool
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success",
			req: func(t *testing.T) *trustpb.UpsertTunnelConnectionRequest {
				return trustpb.UpsertTunnelConnectionRequest_builder{TunnelConnection: newConn(t)}.Build()
			},
			allow: map[check]bool{
				{types.KindTunnelConnection, types.VerbCreate}: true,
				{types.KindTunnelConnection, types.VerbUpdate}: true,
			},
			assertErr: require.NoError,
		},
		{
			name: "missing tunnel connection",
			req: func(t *testing.T) *trustpb.UpsertTunnelConnectionRequest {
				return &trustpb.UpsertTunnelConnectionRequest{}
			},
			allow: map[check]bool{
				{types.KindTunnelConnection, types.VerbCreate}: true,
				{types.KindTunnelConnection, types.VerbUpdate}: true,
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			},
		},
		{
			name: "access denied",
			req: func(t *testing.T) *trustpb.UpsertTunnelConnectionRequest {
				return trustpb.UpsertTunnelConnectionRequest_builder{TunnelConnection: newConn(t)}.Build()
			},
			allow: map[check]bool{
				{types.KindTunnelConnection, types.VerbCreate}: false,
				{types.KindTunnelConnection, types.VerbUpdate}: false,
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newTestPack(t)
			trust := local.NewCAService(p.mem)
			authorizer := &fakeAuthorizer{checker: &fakeChecker{allow: test.allow}}
			service, err := NewService(&ServiceConfig{
				Cache:            trust,
				Backend:          trust,
				Authorizer:       authorizer,
				ScopedAuthorizer: authorizer,
				AuthServer:       &fakeAuthServer{},
				Modules:          modulestest.OSSModules(),
			})
			require.NoError(t, err)

			req := test.req(t)
			resp, err := service.UpsertTunnelConnection(ctx, req)
			test.assertErr(t, err)
			if err != nil {
				return
			}
			require.NotEmpty(t, resp.GetTunnelConnection().GetRevision(), "response should carry the backend-assigned revision")
			require.Equal(t, req.GetTunnelConnection(), resp.GetTunnelConnection())

			stored, err := trust.GetTunnelConnections(ctx, req.GetTunnelConnection().GetClusterName())
			require.NoError(t, err)
			require.Len(t, stored, 1)
			require.Equal(t, req.GetTunnelConnection().GetName(), stored[0].GetName())
			require.Equal(t, resp.GetTunnelConnection().GetRevision(), stored[0].GetRevision())
		})
	}
}

func TestDeleteTunnelConnection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const clusterName = "leaf.example.com"
	const connName = "conn-1"

	seed := func(t *testing.T, trust *local.CA) {
		t.Helper()
		conn, err := types.NewTunnelConnection(connName, types.TunnelConnectionSpecV2{
			ClusterName:   clusterName,
			ProxyName:     "proxy-1",
			LastHeartbeat: time.Now().UTC(),
			Type:          types.ProxyTunnel,
		})
		require.NoError(t, err)
		require.NoError(t, trust.UpsertTunnelConnection(ctx, conn))
	}

	tests := []struct {
		name      string
		req       *trustpb.DeleteTunnelConnectionRequest
		allow     map[check]bool
		assertErr require.ErrorAssertionFunc
		wantGone  bool
	}{
		{
			name: "success",
			req: trustpb.DeleteTunnelConnectionRequest_builder{
				ClusterName:    clusterName,
				ConnectionName: connName,
			}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbDelete}: true},
			assertErr: require.NoError,
			wantGone:  true,
		},
		{
			name:      "missing cluster name",
			req:       trustpb.DeleteTunnelConnectionRequest_builder{ConnectionName: connName}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbDelete}: true},
			assertErr: func(t require.TestingT, err error, _ ...any) { require.True(t, trace.IsBadParameter(err)) },
		},
		{
			name:      "missing connection name",
			req:       trustpb.DeleteTunnelConnectionRequest_builder{ClusterName: clusterName}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbDelete}: true},
			assertErr: func(t require.TestingT, err error, _ ...any) { require.True(t, trace.IsBadParameter(err)) },
		},
		{
			name: "access denied",
			req: trustpb.DeleteTunnelConnectionRequest_builder{
				ClusterName:    clusterName,
				ConnectionName: connName,
			}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbDelete}: false},
			assertErr: func(t require.TestingT, err error, _ ...any) { require.True(t, trace.IsAccessDenied(err)) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newTestPack(t)
			trust := local.NewCAService(p.mem)
			seed(t, trust)
			authorizer := &fakeAuthorizer{checker: &fakeChecker{allow: test.allow}}
			service, err := NewService(&ServiceConfig{
				Cache:            trust,
				Backend:          trust,
				Authorizer:       authorizer,
				ScopedAuthorizer: authorizer,
				AuthServer:       &fakeAuthServer{},
				Modules:          modulestest.OSSModules(),
			})
			require.NoError(t, err)

			_, err = service.DeleteTunnelConnection(ctx, test.req)
			test.assertErr(t, err)

			remaining, listErr := trust.GetTunnelConnections(ctx, clusterName)
			require.NoError(t, listErr)
			if test.wantGone {
				require.Empty(t, remaining)
			} else {
				require.Len(t, remaining, 1)
			}
		})
	}
}

func TestListTunnelConnections(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const leaf1 = "leaf-1.example.com"
	const leaf2 = "leaf-2.example.com"

	seed := func(t *testing.T, trust *local.CA) {
		t.Helper()
		fixtures := []struct {
			cluster, name, proxy string
		}{
			{leaf1, "conn-a", "proxy-1"},
			{leaf1, "conn-b", "proxy-2"},
			{leaf2, "conn-a", "proxy-3"},
		}
		for _, f := range fixtures {
			conn, err := types.NewTunnelConnection(f.name, types.TunnelConnectionSpecV2{
				ClusterName:   f.cluster,
				ProxyName:     f.proxy,
				LastHeartbeat: time.Now().UTC(),
				Type:          types.ProxyTunnel,
			})
			require.NoError(t, err)
			require.NoError(t, trust.UpsertTunnelConnection(ctx, conn))
		}
	}

	type want struct {
		clusters []string
		names    []string
	}

	tests := []struct {
		name      string
		req       *trustpb.ListTunnelConnectionsRequest
		allow     map[check]bool
		assertErr require.ErrorAssertionFunc
		want      want
	}{
		{
			name:      "success no filter",
			req:       &trustpb.ListTunnelConnectionsRequest{},
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbList}: true, {types.KindTunnelConnection, types.VerbRead}: true},
			assertErr: require.NoError,
			want: want{
				clusters: []string{leaf1, leaf1, leaf2},
				names:    []string{"conn-a", "conn-b", "conn-a"},
			},
		},
		{
			name: "success filter by cluster",
			req: trustpb.ListTunnelConnectionsRequest_builder{
				Filter: trustpb.ListTunnelConnectionsFilter_builder{ClusterName: leaf1}.Build(),
			}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbList}: true, {types.KindTunnelConnection, types.VerbRead}: true},
			assertErr: require.NoError,
			want: want{
				clusters: []string{leaf1, leaf1},
				names:    []string{"conn-a", "conn-b"},
			},
		},
		{
			name: "success filter unknown cluster returns empty",
			req: trustpb.ListTunnelConnectionsRequest_builder{
				Filter: trustpb.ListTunnelConnectionsFilter_builder{ClusterName: "nonexistent"}.Build(),
			}.Build(),
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbList}: true, {types.KindTunnelConnection, types.VerbRead}: true},
			assertErr: require.NoError,
		},
		{
			name:      "access denied",
			req:       &trustpb.ListTunnelConnectionsRequest{},
			allow:     map[check]bool{{types.KindTunnelConnection, types.VerbList}: false},
			assertErr: func(t require.TestingT, err error, _ ...any) { require.True(t, trace.IsAccessDenied(err)) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newTestPack(t)
			trust := local.NewCAService(p.mem)
			seed(t, trust)
			authorizer := &fakeAuthorizer{checker: &fakeChecker{allow: test.allow}}
			service, err := NewService(&ServiceConfig{
				Cache:            trust,
				Backend:          trust,
				Authorizer:       authorizer,
				ScopedAuthorizer: authorizer,
				AuthServer:       &fakeAuthServer{},
				Modules:          modulestest.OSSModules(),
			})
			require.NoError(t, err)

			resp, err := service.ListTunnelConnections(ctx, test.req)
			test.assertErr(t, err)
			if err != nil {
				return
			}
			var gotClusters, gotNames []string
			for _, c := range resp.GetTunnelConnections() {
				gotClusters = append(gotClusters, c.GetClusterName())
				gotNames = append(gotNames, c.GetName())
			}
			require.Equal(t, test.want.clusters, gotClusters)
			require.Equal(t, test.want.names, gotNames)
		})
	}
}

func TestListTunnelConnections_Pagination(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const cluster = "leaf.example.com"

	p := newTestPack(t)
	trust := local.NewCAService(p.mem)
	for i := 0; i < 5; i++ {
		conn, err := types.NewTunnelConnection(fmt.Sprintf("conn-%d", i), types.TunnelConnectionSpecV2{
			ClusterName:   cluster,
			ProxyName:     fmt.Sprintf("proxy-%d", i),
			LastHeartbeat: time.Now().UTC(),
			Type:          types.ProxyTunnel,
		})
		require.NoError(t, err)
		require.NoError(t, trust.UpsertTunnelConnection(ctx, conn))
	}

	authorizer := &fakeAuthorizer{checker: &fakeChecker{allow: map[check]bool{
		{types.KindTunnelConnection, types.VerbList}: true,
		{types.KindTunnelConnection, types.VerbRead}: true,
	}}}
	service, err := NewService(&ServiceConfig{
		Cache:            trust,
		Backend:          trust,
		Authorizer:       authorizer,
		ScopedAuthorizer: authorizer,
		AuthServer:       &fakeAuthServer{},
		Modules:          modulestest.OSSModules(),
	})
	require.NoError(t, err)

	var gotNames []string
	var pageToken string
	for {
		resp, err := service.ListTunnelConnections(ctx, trustpb.ListTunnelConnectionsRequest_builder{
			PageSize:  2,
			PageToken: pageToken,
		}.Build())
		require.NoError(t, err)
		for _, c := range resp.GetTunnelConnections() {
			gotNames = append(gotNames, c.GetName())
		}
		if resp.GetNextPageToken() == "" {
			break
		}
		pageToken = resp.GetNextPageToken()
	}
	require.Equal(t, []string{"conn-0", "conn-1", "conn-2", "conn-3", "conn-4"}, gotNames)
}
