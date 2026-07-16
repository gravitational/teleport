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

package kube_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/kube"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const testClusterName = "test-cluster"

func TestKubeClusterService(t *testing.T) {
	ctx := t.Context()
	pack := newBackendPack(t)

	scopedCluster := newKubeCluster(t, "/aa/aa", "kube-cluster", map[string]string{
		"env": "test",
	})
	orthogonalCluster := newKubeCluster(t, "/aa/bb", "kube-cluster", map[string]string{
		"env": "test",
	})
	prodCluster := newKubeCluster(t, "/aa/aa", "prod-cluster", map[string]string{
		"env": "prod",
	})
	unscopedCluster := newKubeCluster(t, "", "unscoped-cluster", map[string]string{
		"env": "test",
	})
	for _, cluster := range []types.KubeCluster{scopedCluster, orthogonalCluster, prodCluster, unscopedCluster} {
		require.NoError(t, pack.kubeService.CreateKubernetesCluster(ctx, cluster))
	}

	reader := newServerForIdentity(t, pack, &services.AccessInfo{
		Username: "reader",
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: "/aa/aa",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/aa/aa": {"/aa/aa": {"/aa::staging-read"}},
			}),
		}.Build(),
	}, nil)
	prodReader := newServerForIdentity(t, pack, &services.AccessInfo{
		Username: "prod-reader",
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: "/aa/aa",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/aa/aa": {"/aa/aa": {"/aa::staging-prod-read"}},
			}),
		}.Build(),
	}, nil)
	deleter := newServerForIdentity(t, pack, &services.AccessInfo{
		Username: "deleter",
		ScopePin: scopesv1.Pin_builder{
			Kind:  scopesv1.PinKind_PIN_KIND_USER,
			Scope: "/aa/aa",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/aa/aa": {"/aa/aa": {"/aa::staging-delete"}},
			}),
		}.Build(),
	}, nil)

	unscoped := newServerForIdentity(t, pack, &services.AccessInfo{
		Username: "unscoped",
	}, nil)

	unscopedDenier := newServerForIdentity(t, pack, &services.AccessInfo{
		Username: "unscoped-denier",
	}, errors.New("test failure"))

	t.Run("get scoped cluster", func(t *testing.T) {
		res, err := reader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: scopedCluster.GetScope(),
			Name:  scopedCluster.GetName(),
		}.Build())
		require.NoError(t, err)
		require.Equal(t, scopedCluster.GetName(), res.GetCluster().GetName())
		require.Equal(t, scopedCluster.GetScope(), res.GetCluster().GetScope())
	})

	t.Run("get denies unscoped and orthogonal clusters", func(t *testing.T) {
		_, err := reader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Name: unscopedCluster.GetName(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)

		_, err = reader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: orthogonalCluster.GetScope(),
			Name:  orthogonalCluster.GetName(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	})

	t.Run("get applies kube label access", func(t *testing.T) {
		_, err := reader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: prodCluster.GetScope(),
			Name:  prodCluster.GetName(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)

		res, err := prodReader.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: prodCluster.GetScope(),
			Name:  prodCluster.GetName(),
		}.Build())
		require.NoError(t, err)
		require.Equal(t, prodCluster.GetName(), res.GetCluster().GetName())
	})

	t.Run("list filters inaccessible clusters", func(t *testing.T) {
		res, err := reader.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize: 10,
		}.Build())
		require.NoError(t, err)
		require.Empty(t, res.GetNextPageToken())
		require.Len(t, res.GetClusters(), 1)
		require.Equal(t, scopedCluster.GetName(), res.GetClusters()[0].GetName())
		require.Equal(t, scopedCluster.GetScope(), res.GetClusters()[0].GetScope())
	})

	t.Run("list applies kube label access", func(t *testing.T) {
		res, err := prodReader.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize: 10,
		}.Build())
		require.NoError(t, err)
		require.Empty(t, res.GetNextPageToken())
		require.Len(t, res.GetClusters(), 1)
		require.Equal(t, prodCluster.GetName(), res.GetClusters()[0].GetName())
		require.Equal(t, prodCluster.GetScope(), res.GetClusters()[0].GetScope())
	})

	t.Run("list paginates accessible clusters after filtering", func(t *testing.T) {
		secondCluster := newKubeCluster(t, "/aa/aa", "second-cluster", map[string]string{
			"env": "test",
		})
		require.NoError(t, pack.kubeService.CreateKubernetesCluster(ctx, secondCluster))
		t.Cleanup(func() {
			pack.kubeService.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
				Scope: secondCluster.GetScope(),
				Name:  secondCluster.GetName(),
			}.Build())
		})

		firstPage, err := reader.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize: 1,
		}.Build())
		require.NoError(t, err)
		require.Len(t, firstPage.GetClusters(), 1)
		require.NotEmpty(t, firstPage.GetNextPageToken())

		secondPage, err := reader.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize:  1,
			PageToken: firstPage.GetNextPageToken(),
		}.Build())
		require.NoError(t, err)
		require.Len(t, secondPage.GetClusters(), 1)
		require.Empty(t, secondPage.GetNextPageToken())
		require.NotEqual(t, firstPage.GetClusters()[0].GetName(), secondPage.GetClusters()[0].GetName())
	})

	t.Run("reader cannot delete cluster", func(t *testing.T) {
		_, err := reader.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
			Scope: scopedCluster.GetScope(),
			Name:  scopedCluster.GetName(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	})

	t.Run("deleter cannot delete orthogonal cluster", func(t *testing.T) {
		_, err := deleter.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
			Scope: orthogonalCluster.GetScope(),
			Name:  orthogonalCluster.GetName(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	})

	t.Run("deleter can delete scoped cluster", func(t *testing.T) {
		clusterForDelete := newKubeCluster(t, "/aa/aa", "delete-cluster", map[string]string{
			"env": "test",
		})
		require.NoError(t, pack.kubeService.CreateKubernetesCluster(ctx, clusterForDelete))

		_, err := deleter.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.NoError(t, err)

		_, err = pack.kubeService.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)
	})

	t.Run("unscoped can get a cluster", func(t *testing.T) {
		res, err := unscoped.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: scopedCluster.GetScope(),
			Name:  scopedCluster.GetName(),
		}.Build())
		require.NoError(t, err)
		require.Equal(t, scopedCluster.GetName(), res.GetCluster().GetName())
		require.Equal(t, scopedCluster.GetScope(), res.GetCluster().GetScope())
	})

	t.Run("unscoped-denier can not get a cluster", func(t *testing.T) {
		_, err := unscopedDenier.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: scopedCluster.GetScope(),
			Name:  scopedCluster.GetName(),
		}.Build())
		require.Error(t, err)
	})

	t.Run("unscoped can delete a cluster", func(t *testing.T) {
		clusterForDelete := newKubeCluster(t, "/aa/aa", "delete-cluster", map[string]string{
			"env": "test",
		})
		require.NoError(t, pack.kubeService.CreateKubernetesCluster(ctx, clusterForDelete))

		_, err := unscoped.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.NoError(t, err)

		_, err = pack.kubeService.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.True(t, trace.IsNotFound(err), "expected not found, got %v", err)
	})

	t.Run("unscoped-denier can not delete a cluster", func(t *testing.T) {
		clusterForDelete := newKubeCluster(t, "/aa/aa", "delete-cluster", map[string]string{
			"env": "test",
		})
		require.NoError(t, pack.kubeService.CreateKubernetesCluster(ctx, clusterForDelete))

		_, err := unscopedDenier.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.Error(t, err)
		t.Cleanup(func() {
			pack.kubeService.DeleteKubeCluster(ctx, kubev1.DeleteKubeClusterRequest_builder{
				Scope: clusterForDelete.GetScope(),
				Name:  clusterForDelete.GetName(),
			}.Build())
		})

		cluster, err := pack.kubeService.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{
			Scope: clusterForDelete.GetScope(),
			Name:  clusterForDelete.GetName(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, cluster)
	})

	t.Run("unscoped can list all clusters", func(t *testing.T) {
		res, err := unscoped.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize: 10,
		}.Build())
		require.NoError(t, err)
		require.Empty(t, res.GetNextPageToken())
		require.Len(t, res.GetClusters(), 4)
	})

	t.Run("unscoped-denier can list no clusters", func(t *testing.T) {
		_, err := unscopedDenier.ListKubeClusters(ctx, kubev1.ListKubeClustersRequest_builder{
			PageSize: 10,
		}.Build())
		require.Error(t, err)
	})
}

func newKubeCluster(t *testing.T, scope string, name string, labels map[string]string) *types.KubernetesClusterV3 {
	t.Helper()

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	cluster.Scope = scope

	return cluster
}

func newServerForIdentity(t *testing.T, bk *backendPack, accessInfo *services.AccessInfo, unscopedErr error) *kube.Server {
	t.Helper()

	authorizer := newFakeAuthorizer(t, newFakeAccessChecker(unscopedErr))
	if accessInfo.ScopePin != nil {
		authorizer = newFakeScopedAuthorizer(t, accessInfo, bk.service)
	}
	srv, err := kube.New(&kube.Config{
		ScopedAuthorizer: authorizer,
		Logger:           logtest.NewLogger(),
		ClusterReader:    bk.kubeService,
		ClusterWriter:    bk.kubeService,
	})
	require.NoError(t, err)
	return srv
}

type backendPack struct {
	backend     backend.Backend
	service     *local.ScopedAccessService
	kubeService *local.KubernetesService
}

func (p *backendPack) Close() {
	p.backend.Close()
}

func newBackendPack(t *testing.T) *backendPack {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { backend.Close() })

	service := local.NewScopedAccessService(backend)
	kubeService, err := local.NewKubernetesService(backend)
	require.NoError(t, err)

	roles := []*scopedaccessv1.ScopedRole{
		newScopedKubeRole("staging-read", []string{types.VerbRead, types.VerbList}, map[string][]string{
			"env": []string{"test"},
		}),
		newScopedKubeRole("staging-prod-read", []string{types.VerbRead, types.VerbList}, map[string][]string{
			"env": []string{"prod"},
		}),
		newScopedKubeRole("staging-delete", []string{types.VerbDelete}, map[string][]string{
			types.Wildcard: []string{types.Wildcard},
		}),
	}

	for _, role := range roles {
		_, err := service.CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: role,
		}.Build())
		require.NoError(t, err)
	}

	return &backendPack{
		backend:     backend,
		service:     service,
		kubeService: kubeService,
	}
}

func newScopedKubeRole(name string, verbs []string, kubeLabels map[string][]string) *scopedaccessv1.ScopedRole {
	labels := make([]*labelv1.Label, 0, len(kubeLabels))
	for name, values := range kubeLabels {
		labels = append(labels, labelv1.Label_builder{
			Name:   name,
			Values: values,
		}.Build())
	}
	return scopedaccessv1.ScopedRole_builder{
		Kind: scopedaccess.KindScopedRole,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Scope: "/aa",
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{"/aa/aa"},
			Rules: []*scopedaccessv1.ScopedRule{
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{types.KindKubernetesCluster},
					Verbs:     verbs,
				}.Build(),
			},
			Kube: scopedaccessv1.ScopedRoleKube_builder{
				Labels: labels,
			}.Build(),
		}.Build(),
		Version: types.V1,
	}.Build()
}

type fakeAuthorizer struct {
	ctx *authz.ScopedContext
}

func (a *fakeAuthorizer) AuthorizeScoped(ctx context.Context) (*authz.ScopedContext, error) {
	return a.ctx, nil
}

func newFakeScopedAuthorizer(t *testing.T, accessInfo *services.AccessInfo, reader services.ScopedRoleReader) *fakeAuthorizer {
	t.Helper()

	scopedCtx, err := services.NewScopedAccessCheckerContext(t.Context(), accessInfo, testClusterName, reader)
	require.NoError(t, err)

	return &fakeAuthorizer{
		ctx: &authz.ScopedContext{
			User: &types.UserV2{
				Metadata: types.Metadata{
					Name: accessInfo.Username,
				},
			},
			CheckerContext: scopedCtx,
		},
	}
}

type fakeAccessChecker struct {
	services.AccessChecker
	err error
}

func (ac *fakeAccessChecker) CheckAccess(r services.AccessCheckable, state services.AccessState, matchers ...services.RoleMatcher) error {
	return ac.err
}

func (ac *fakeAccessChecker) CheckAccessToRule(ctx services.RuleContext, namespace, rule, verb string) error {
	return ac.err
}

func (ac *fakeAccessChecker) GuessIfAccessIsPossible(ctx services.RuleContext, namespace, resource, verb string) error {
	return ac.err
}

func newFakeAccessChecker(err error) *fakeAccessChecker {
	return &fakeAccessChecker{err: err}
}

func newFakeAuthorizer(t *testing.T, checker services.AccessChecker) *fakeAuthorizer {
	t.Helper()

	return &fakeAuthorizer{
		ctx: authz.ScopedContextFromUnscopedContext(&authz.Context{
			Checker: checker,
		}),
	}
}
