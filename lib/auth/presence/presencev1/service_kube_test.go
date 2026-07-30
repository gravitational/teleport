// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package presencev1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestPresenceServiceKubeClusters(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	kube, err := local.NewKubernetesService(bk)
	require.NoError(t, err)

	roles := fakeScopedRoleReader{roles: map[string]*accessv1.ScopedRole{}}
	scopedReadRole := roles.createScopedRole("read-role", types.VerbRead)
	scopedListRole := roles.createScopedRole("list-role", types.VerbRead, types.VerbList)
	scopedDeleteRole := roles.createScopedRole("delete-role", types.VerbDelete)

	const (
		readUser   = "read-user"
		listUser   = "list-user"
		deleteUser = "delete-user"
	)

	scopedCluster := newKubeCluster(t, kube, testScope, "kube-cluster", map[string]string{
		"env": "test",
	})
	orthogonalCluster := newKubeCluster(t, kube, testOrthogonalScope, "kube-cluster", map[string]string{
		"env": "test",
	})
	unscopedCluster := newKubeCluster(t, kube, "", "unscoped-cluster", map[string]string{
		"env": "test",
	})

	t.Run("GetKubeCluster", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			authorizer authz.ScopedAuthorizer
			cluster    types.KubeCluster
			shouldFail bool
		}{
			{
				name:       "unscoped read-user fetching unscoped kube cluster",
				authorizer: newFakeAuthorizer(t, readUser, types.KindKubernetesCluster, types.VerbRead),
				cluster:    unscopedCluster,
			},
			{
				name:       "unscoped read-user fetching scoped kube cluster",
				authorizer: newFakeAuthorizer(t, readUser, types.KindKubernetesCluster, types.VerbRead),
				cluster:    scopedCluster,
			},
			{
				name:       "unscoped delete-user fetching unscoped kube cluster",
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindKubernetesCluster, types.VerbDelete),
				cluster:    unscopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:       "unscoped delete-user fetching scoped kube cluster",
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindKubernetesCluster, types.VerbDelete),
				cluster:    scopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:       "scoped read-user fetching scoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    scopedCluster,
			},
			{
				name:       "scoped read-user fetching orthogonal scoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user fetching unscoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user fetching unscoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user fetching scoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    scopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:       "scoped delete-user fetching orthogonal scoped kube cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				srv := newPresenceService(t, backend{kube: kube}, tt.authorizer)
				res, err := srv.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
					Name:  tt.cluster.GetName(),
					Scope: tt.cluster.GetScope(),
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Empty(t, cmp.Diff(tt.cluster, res.GetCluster()))
				}
			})
		}
	})

	allClusters := []types.KubeCluster{unscopedCluster, scopedCluster, orthogonalCluster}
	t.Run("ListKubeClusters", func(t *testing.T) {
		for _, tt := range []struct {
			name             string
			authorizer       authz.ScopedAuthorizer
			req              *presencev1pb.ListKubeClustersRequest
			expectedClusters []types.KubeCluster
			shouldFail       bool
		}{
			{
				name:       "unscoped list-user listing all clusters without scope filter",
				authorizer: newFakeAuthorizer(t, listUser, types.KindKubernetesCluster, types.VerbRead, types.VerbList),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedClusters: []types.KubeCluster{unscopedCluster},
			},
			{
				name:       "unscoped list-user listing all clusters filtering for all scopes",
				authorizer: newFakeAuthorizer(t, listUser, types.KindKubernetesCluster, types.VerbRead, types.VerbList),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build(),
				expectedClusters: allClusters,
			},
			{
				name:       "unscoped list-user listing clusters with scope filter",
				authorizer: newFakeAuthorizer(t, listUser, types.KindKubernetesCluster, types.VerbRead, types.VerbList),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:       "scoped list-user listing all clusters",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:       "scoped list-user listing clusters with scope filter",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:       "scoped list-user listing clusters with orthogonal scope filter",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testOrthogonalScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				srv := newPresenceService(t, backend{kube: kube}, tt.authorizer)
				res, err := srv.ListKubeClusters(t.Context(), tt.req)
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.ElementsMatch(t, tt.expectedClusters, res.GetClusters())
				}
			})
		}
	})

	t.Run("DeleteKubeCluster", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name       string
			authorizer authz.ScopedAuthorizer
			cluster    types.KubeCluster
			shouldFail bool
		}{
			{
				name:       "unscoped read-user deleting unscoped cluster",
				authorizer: newFakeAuthorizer(t, readUser, types.KindKubernetesCluster, types.VerbRead),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting scoped cluster",
				authorizer: newFakeAuthorizer(t, readUser, types.KindKubernetesCluster, types.VerbRead),
				cluster:    scopedCluster,
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting orthogonal cluster",
				authorizer: newFakeAuthorizer(t, readUser, types.KindKubernetesCluster, types.VerbRead),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting unscoped cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting scoped cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    scopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting orthogonal cluster",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting unscoped cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting orthogonal cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting scoped cluster",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				cluster:    scopedCluster,
			},
			{
				name:       "unscoped delete-user deleting unscoped cluster",
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindKubernetesCluster, types.VerbDelete),
				cluster:    unscopedCluster,
			},
			{
				name:       "unscoped delete-user deleting orthogonal cluster",
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindKubernetesCluster, types.VerbDelete),
				cluster:    orthogonalCluster,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				srv := newPresenceService(t, backend{kube: kube}, tt.authorizer)
				_, err := srv.DeleteKubeCluster(t.Context(), presencev1pb.DeleteKubeClusterRequest_builder{
					Name:  tt.cluster.GetName(),
					Scope: tt.cluster.GetScope(),
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func newKubeCluster(t *testing.T, srv *local.KubernetesService, scope, name string, labels map[string]string) types.KubeCluster {
	t.Helper()

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	cluster.Scope = scope

	err = srv.CreateKubernetesCluster(t.Context(), cluster)
	require.NoError(t, err)
	res, err := srv.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
		Name:  name,
		Scope: scope,
	}.Build())
	require.NoError(t, err)

	return res
}
