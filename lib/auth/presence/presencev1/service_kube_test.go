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
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestPresenceServiceKubeClusters(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	kube, err := local.NewKubernetesService(bk)
	require.NoError(t, err)

	roles := fakeScopedRoleReader{roles: map[string]*scopedaccessv1.ScopedRole{}}
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

// TestKubeClusterWithSecrets verifies the behavior of the with_secrets flag.
func TestKubeClusterWithSecrets(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"kube-reader",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindKubernetesCluster},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		},
	)
	require.NoError(t, err)

	const kubeconfig = "kubeconfig-payload"
	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "dynamic-cluster",
		Labels: map[string]string{"env": "test"},
	}, types.KubernetesClusterSpecV3{
		Kubeconfig: []byte(kubeconfig),
	})
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateKubernetesCluster(t.Context(), cluster))

	client, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	t.Run("GetKubeCluster omits the kubeconfig by default", func(t *testing.T) {
		got, err := client.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
			Name: "dynamic-cluster",
		}.Build())
		require.NoError(t, err)
		require.Empty(t, got.GetKubeconfig())
		// the rest of the resource is still there.
		require.Equal(t, "dynamic-cluster", got.GetName())
		require.Equal(t, "test", got.GetAllLabels()["env"])
	})

	t.Run("GetKubeCluster returns the kubeconfig with_secrets", func(t *testing.T) {
		got, err := client.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
			Name:        "dynamic-cluster",
			WithSecrets: true,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, kubeconfig, string(got.GetKubeconfig()))
	})

	t.Run("ListKubeClusters omits the kubeconfig by default", func(t *testing.T) {
		got, _, err := client.ListKubeClusters(t.Context(), presencev1pb.ListKubeClustersRequest_builder{}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, got)
		for _, c := range got {
			require.Empty(t, c.GetKubeconfig(), "cluster %q", c.GetName())
		}
	})

	t.Run("ListKubeClusters returns kubeconfigs with_secrets", func(t *testing.T) {
		got, _, err := client.ListKubeClusters(t.Context(), presencev1pb.ListKubeClustersRequest_builder{
			WithSecrets: true,
		}.Build())
		require.NoError(t, err)
		found := false
		for _, c := range got {
			if c.GetName() == "dynamic-cluster" {
				require.Equal(t, kubeconfig, string(c.GetKubeconfig()))
				found = true
			}
		}
		require.True(t, found, "expected to find dynamic-cluster in %v", got)
	})

	// The methods below are the ones outdated clients reach. They predate with_secrets and must
	// always be secrets-inclusive.
	t.Run("legacy GetKubernetesCluster is secret-inclusive", func(t *testing.T) {
		//nolint:staticcheck // deliberately exercising the deprecated compatibility path
		got, err := client.GetKubernetesCluster(t.Context(), "dynamic-cluster")
		require.NoError(t, err)
		require.Equal(t, kubeconfig, string(got.GetKubeconfig()))
	})

	t.Run("legacy ListKubernetesClusters is secret-inclusive", func(t *testing.T) {
		//nolint:staticcheck // deliberately exercising the deprecated compatibility path
		got, _, err := client.ListKubernetesClusters(t.Context(), 0, "")
		require.NoError(t, err)
		found := false
		for _, c := range got {
			if c.GetName() == "dynamic-cluster" {
				require.Equal(t, kubeconfig, string(c.GetKubeconfig()))
				found = true
			}
		}
		require.True(t, found, "expected to find dynamic-cluster in %v", got)
	})

	t.Run("legacy GetKubernetesClusters is secret-inclusive", func(t *testing.T) {
		//nolint:staticcheck // deliberately exercising the deprecated compatibility path
		got, err := client.GetKubernetesClusters(t.Context())
		require.NoError(t, err)
		found := false
		for _, c := range got {
			if c.GetName() == "dynamic-cluster" {
				require.Equal(t, kubeconfig, string(c.GetKubeconfig()))
				found = true
			}
		}
		require.True(t, found, "expected to find dynamic-cluster in %v", got)
	})
}

// TestScopedKubeClusterWithSecretsAuthz verifies that a scoped identity must hold the
// `secrets` verb to retrieve a kube cluster's kubeconfig, and that holding only `read` still permits
// the censored read.
func TestScopedKubeClusterWithSecretsAuthz(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	const (
		parentScope = "/aa"
		scope       = "/aa/aa"
		kubeconfig  = "kubeconfig-payload"
	)

	createScopedRole := func(name string, verbs []string) *scopedaccessv1.ScopedRole {
		rsp, err := srv.Auth().ScopedAccess().CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:    scopedaccess.KindScopedRole,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: name,
				}.Build(),
				Scope: parentScope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{scope},
					// the kube block is required for the CanAccessCluster checks.
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
		return rsp.GetRole()
	}

	createUserWithScopedRole := func(username string, verbs []string) *authclient.Client {
		user, _, err := authtest.CreateUserAndRole(srv.Auth(), username, []string{}, nil)
		require.NoError(t, err)

		role := createScopedRole(username+"-role", verbs)
		sra, err := srv.Auth().ScopedAccess().CreateScopedRoleAssignment(t.Context(), scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				SubKind: scopedaccess.SubKindDynamic,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Scope: scope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: user.GetName(),
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  scopes.QualifiedName{Scope: parentScope, Name: role.GetMetadata().GetName()}.String(),
							Scope: scope,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		waitForSRACache(t, srv, sra.GetAssignment())

		client, err := srv.NewClient(authtest.TestScopedUser(user.GetName(), scope))
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })
		return client
	}

	// a scoped identity granted the secret-exclusive read verb.
	readClient := createUserWithScopedRole("scoped-reader", []string{
		scopedaccess.List.String(), scopedaccess.Read.String(),
	})
	// a scoped identity that additionally opted in to secret material.
	secretsClient := createUserWithScopedRole("scoped-secrets", []string{
		scopedaccess.List.String(), scopedaccess.Read.String(), scopedaccess.Secrets.String(),
	})

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "dynamic-cluster",
		Labels: map[string]string{"env": "test"},
	}, types.KubernetesClusterSpecV3{
		Kubeconfig: []byte(kubeconfig),
	}, types.KubeClusterWithScope(scope))
	require.NoError(t, err)
	require.NoError(t, srv.Auth().CreateKubernetesCluster(t.Context(), cluster))

	t.Run("read verb permits the censored read", func(t *testing.T) {
		got, err := readClient.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
			Scope: scope,
			Name:  "dynamic-cluster",
		}.Build())
		require.NoError(t, err)
		require.Empty(t, got.GetKubeconfig())
		require.Equal(t, "dynamic-cluster", got.GetName())
	})

	t.Run("read verb is denied with_secrets", func(t *testing.T) {
		_, err := readClient.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
			Scope:       scope,
			Name:        "dynamic-cluster",
			WithSecrets: true,
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	})

	t.Run("secrets verb permits with_secrets", func(t *testing.T) {
		got, err := secretsClient.GetKubeCluster(t.Context(), presencev1pb.GetKubeClusterRequest_builder{
			Scope:       scope,
			Name:        "dynamic-cluster",
			WithSecrets: true,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, kubeconfig, string(got.GetKubeconfig()))
	})

	t.Run("list is censored without the secrets verb", func(t *testing.T) {
		got, _, err := readClient.ListKubeClusters(t.Context(), presencev1pb.ListKubeClustersRequest_builder{
			ScopeFilter: scopesv1.Filter_builder{Scope: scope, Mode: scopesv1.Mode_MODE_EXACT}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, got)
		for _, c := range got {
			require.Empty(t, c.GetKubeconfig(), "cluster %q", c.GetName())
		}
	})

	// a quirk of how scoped access works is that one can have a mix of secret-inclusive and secret-exclusive permissions
	// for different scopes.  As such we must filter instead of blanket deny when the with_secrets flag is set.
	t.Run("with_secrets list filters rather than denying", func(t *testing.T) {
		got, _, err := readClient.ListKubeClusters(t.Context(), presencev1pb.ListKubeClustersRequest_builder{
			ScopeFilter: scopesv1.Filter_builder{Scope: scope, Mode: scopesv1.Mode_MODE_EXACT}.Build(),
			WithSecrets: true,
		}.Build())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("secrets verb permits a with_secrets list", func(t *testing.T) {
		got, _, err := secretsClient.ListKubeClusters(t.Context(), presencev1pb.ListKubeClustersRequest_builder{
			ScopeFilter: scopesv1.Filter_builder{Scope: scope, Mode: scopesv1.Mode_MODE_EXACT}.Build(),
			WithSecrets: true,
		}.Build())
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, kubeconfig, string(got[0].GetKubeconfig()))
	})
}
