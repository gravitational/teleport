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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/access"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

const testClusterName = "test-cluster"

func TestPresenceServiceKubeClusters(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	readUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"read-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindKubernetesCluster},
				Verbs:     []string{types.VerbRead},
			},
		},
	)
	require.NoError(t, err)

	listUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"list-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindKubernetesCluster},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		},
	)
	require.NoError(t, err)

	deleteUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"delete-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindKubernetesCluster},
				Verbs:     []string{types.VerbDelete},
			},
		},
	)
	require.NoError(t, err)

	const (
		parentScope     = "/aa"
		scope           = "/aa/aa"
		orthogonalScope = "/aa/bb"
	)
	createScopedRole := func(name string, verbs []string) *scopedaccessv1.ScopedRole {
		scopedRole, err := srv.Auth().ScopedAccess().CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:    access.KindScopedRole,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: name,
				}.Build(),
				Scope: parentScope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{scope, orthogonalScope},
					// need kube block because of the CanAccessKubeCluster checks
					Kube: scopedaccessv1.ScopedRoleKube_builder{
						Labels: []*labelv1.Label{
							labelv1.Label_builder{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							}.Build(),
						},
						Resources: []*scopedaccessv1.KubeResource{
							scopedaccessv1.KubeResource_builder{
								Kind:      types.Wildcard,
								Name:      types.Wildcard,
								Namespace: types.Wildcard,
								ApiGroup:  types.Wildcard,
								Verbs:     []string{types.Wildcard},
							}.Build(),
						},
					}.Build(),
					Rules: []*scopedaccessv1.ScopedRule{
						scopedaccessv1.ScopedRule_builder{
							Resources: []string{types.KindKubernetesCluster},
							Verbs:     verbs,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		return scopedRole.GetRole()
	}

	scopedReadRole := createScopedRole("read-role", []string{types.VerbRead})
	scopedListRole := createScopedRole("list-role", []string{types.VerbRead, types.VerbList})
	scopedDeleteRole := createScopedRole("delete-role", []string{types.VerbDelete})

	createAssignment := func(role *scopedaccessv1.ScopedRole, username, assignedScope string) *scopedaccessv1.ScopedRoleAssignment {
		sra, err := srv.Auth().ScopedAccess().CreateScopedRoleAssignment(t.Context(), scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
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

	waitForSRACache(
		t,
		srv,
		createAssignment(scopedReadRole, readUser.GetName(), scope),
		createAssignment(scopedListRole, listUser.GetName(), scope),
		createAssignment(scopedDeleteRole, deleteUser.GetName(), scope),
	)

	scopedCluster := newKubeCluster(t, srv.Auth(), scope, "kube-cluster", map[string]string{
		"env": "test",
	})
	orthogonalCluster := newKubeCluster(t, srv.Auth(), orthogonalScope, "kube-cluster", map[string]string{
		"env": "test",
	})
	// prodCluster := newKubeCluster(t, srv.Auth(), "/aa/aa", "prod-cluster", map[string]string{
	// 	"env": "prod",
	// })
	unscopedCluster := newKubeCluster(t, srv.Auth(), "", "unscoped-cluster", map[string]string{
		"env": "test",
	})

	newClient := func(t *testing.T, identity authtest.TestIdentity) *authclient.Client {
		client, err := srv.NewClient(identity)
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })
		return client
	}

	t.Run("GetKubeCluster", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			client     *authclient.Client
			cluster    types.KubeCluster
			shouldFail bool
		}{
			{
				name:    "unscoped read-user fetching unscoped kube cluster",
				client:  newClient(t, authtest.TestUser(readUser.GetName())),
				cluster: unscopedCluster,
			},
			{
				name:    "unscoped read-user fetching scoped kube cluster",
				client:  newClient(t, authtest.TestUser(readUser.GetName())),
				cluster: scopedCluster,
			},
			{
				name:    "unscoped delete-user fetching unscoped kube cluster",
				client:  newClient(t, authtest.TestUser(deleteUser.GetName())),
				cluster: unscopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:    "unscoped delete-user fetching scoped kube cluster",
				client:  newClient(t, authtest.TestUser(deleteUser.GetName())),
				cluster: scopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:    "scoped read-user fetching scoped kube cluster",
				client:  newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster: scopedCluster,
			},
			{
				name:       "scoped read-user fetching orthogonal scoped kube cluster",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user fetching unscoped kube cluster",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user fetching unscoped kube cluster",
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:    "scoped delete-user fetching scoped kube cluster",
				client:  newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster: scopedCluster,
				// default implicit role provides read access
				shouldFail: false,
			},
			{
				name:       "scoped delete-user fetching orthogonal scoped kube cluster",
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				res, err := tt.client.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
					Name:  tt.cluster.GetName(),
					Scope: tt.cluster.GetScope(),
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Empty(t, cmp.Diff(tt.cluster, res))
				}
			})
		}
	})

	allClusters := []types.KubeCluster{unscopedCluster, scopedCluster, orthogonalCluster}
	t.Run("ListKubeClusters", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name             string
			client           *authclient.Client
			req              *presencev1pb.ListKubeClustersRequest
			expectedClusters []types.KubeCluster
			shouldFail       bool
		}{
			{
				name:   "unscoped list-user listing all clusters without scope filter",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedClusters: []types.KubeCluster{unscopedCluster},
			},
			{
				name:   "unscoped list-user listing all clusters filtering for all scopes",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build(),
				expectedClusters: allClusters,
			},
			{
				name:   "unscoped list-user listing clusters with scope filter",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: scope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:   "scoped list-user listing all clusters",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:   "scoped list-user listing clusters with scope filter",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: scope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedClusters: []types.KubeCluster{scopedCluster},
			},
			{
				name:   "scoped list-user listing clusters with orthogonal scope filter",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListKubeClustersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: orthogonalScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				clusters, _, err := tt.client.ListKubeClusters(t.Context(), tt.req)
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.ElementsMatch(t, tt.expectedClusters, clusters)
				}
			})
		}
	})

	t.Run("DeleteKubeCluster", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name       string
			client     *authclient.Client
			cluster    types.KubeCluster
			shouldFail bool
		}{
			{
				name:       "unscoped read-user deleting unscoped cluster",
				client:     newClient(t, authtest.TestUser(readUser.GetName())),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting scoped cluster",
				client:     newClient(t, authtest.TestUser(readUser.GetName())),
				cluster:    scopedCluster,
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting orthogonal cluster",
				client:     newClient(t, authtest.TestUser(readUser.GetName())),
				cluster:    scopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting unscoped cluster",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting scoped cluster",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster:    scopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting scoped cluster",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting unscoped cluster",
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster:    unscopedCluster,
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting orthogonal cluster",
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster:    orthogonalCluster,
				shouldFail: true,
			},
			{
				name:    "scoped delete-user deleting scoped cluster",
				client:  newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				cluster: scopedCluster,
			},
			{
				name:    "unscoped delete-user deleting unscoped cluster",
				client:  newClient(t, authtest.TestUser(deleteUser.GetName())),
				cluster: unscopedCluster,
			},
			{
				name:    "unscoped delete-user deleting orthogonal cluster",
				client:  newClient(t, authtest.TestUser(deleteUser.GetName())),
				cluster: orthogonalCluster,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.client.DeleteKubeCluster(t.Context(), presencev1pb.DeleteKubeClusterRequest_builder{
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

func newKubeCluster(t *testing.T, srv *auth.Server, scope, name string, labels map[string]string) types.KubeCluster {
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
