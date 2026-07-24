// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"
	"iter"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// TestKubernetes tests that CRUD operations on kubernetes clusters resources are
// replicated from the backend to the cache.
func TestKubernetes(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy, ignoreRangeEndKey())
	t.Cleanup(p.Close)

	t.Run("GetKubernetesClusters", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeCluster]{
			newResource: func(name string) (types.KubeCluster, error) {
				return types.NewKubernetesClusterV3(types.Metadata{
					Name: name,
				}, types.KubernetesClusterSpecV3{})
			},
			create:    p.kubernetes.CreateKubernetesCluster,
			list:      getAllAdapter(p.kubernetes.GetKubernetesClusters),
			cacheGet:  cacheGetKubeClusterWithScope(p.cache, ""),
			cacheList: getAllAdapter(p.cache.GetKubernetesClusters),
			update:    p.kubernetes.UpdateKubernetesCluster,
			deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
		}, withSkipPaginationTest())
	})

	t.Run("ListKubernetesClusters", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeCluster]{
			newResource: func(name string) (types.KubeCluster, error) {
				return types.NewKubernetesClusterV3(types.Metadata{
					Name: name,
				}, types.KubernetesClusterSpecV3{})
			},
			create: p.kubernetes.CreateKubernetesCluster,
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
				return p.kubernetes.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				}.Build())
			},
			cacheGet:  cacheGetKubeClusterWithScope(p.cache, ""),
			cacheList: cacheListKubeClustersWithScopeFilter(p.cache, nil),
			update:    p.kubernetes.UpdateKubernetesCluster,
			deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
			Range: func(ctx context.Context, start, end string) iter.Seq2[types.KubeCluster, error] {
				return p.kubernetes.RangeKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
					PageToken: start,
				}.Build())
			},
			cacheRange: cacheRangeKubeClustersWithScopeFilter(p.cache, nil),
		})
	})

	t.Run("GetKubernetesClusters scoped", func(t *testing.T) {
		const scope = "/test"
		testResources(t, p, testFuncs[types.KubeCluster]{
			newResource: func(name string) (types.KubeCluster, error) {
				return types.NewKubernetesClusterV3(types.Metadata{
					Name: name,
				}, types.KubernetesClusterSpecV3{}, types.KubeClusterWithScope(scope))
			},
			create:    p.kubernetes.CreateKubernetesCluster,
			list:      getAllAdapter(p.kubernetes.GetKubernetesClusters),
			cacheGet:  cacheGetKubeClusterWithScope(p.cache, scope),
			cacheList: getAllAdapter(p.cache.GetKubernetesClusters),
			update:    p.kubernetes.UpdateKubernetesCluster,
			deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
		}, withSkipPaginationTest())
	})

	t.Run("ListKubeClusters scoped", func(t *testing.T) {
		const scope = "/test"
		scopeFilter := scopesv1.Filter_builder{
			Scope: scope,
			Mode:  scopesv1.Mode_MODE_EXACT,
		}.Build()
		testResources(t, p, testFuncs[types.KubeCluster]{
			newResource: func(name string) (types.KubeCluster, error) {
				return types.NewKubernetesClusterV3(types.Metadata{
					Name: name,
				}, types.KubernetesClusterSpecV3{}, types.KubeClusterWithScope(scope))
			},
			create: p.kubernetes.CreateKubernetesCluster,
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
				return p.kubernetes.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
					ScopeFilter: scopesv1.Filter_builder{
						Mode:  scopesv1.Mode_MODE_EXACT,
						Scope: scope,
					}.Build(),
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				}.Build())
			},
			cacheGet:  cacheGetKubeClusterWithScope(p.cache, scope),
			cacheList: cacheListKubeClustersWithScopeFilter(p.cache, scopeFilter),
			update:    p.kubernetes.UpdateKubernetesCluster,
			deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
			Range: func(ctx context.Context, start, end string) iter.Seq2[types.KubeCluster, error] {
				return p.kubernetes.RangeKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
					PageToken: start,
				}.Build())
			},
			cacheRange: cacheRangeKubeClustersWithScopeFilter(p.cache, scopeFilter),
		})
	})
}

func cacheGetKubeClusterWithScope(cache *Cache, scope string) func(context.Context, string) (types.KubeCluster, error) {
	return func(ctx context.Context, name string) (types.KubeCluster, error) {
		return cache.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
			Scope: scope,
			Name:  name,
		}.Build())
	}
}

func cacheListKubeClustersWithScopeFilter(cache *Cache, scopeFilter *scopesv1.Filter) func(context.Context, int, string) ([]types.KubeCluster, string, error) {
	return func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
		return cache.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
			PageSize:    int32(pageSize),
			PageToken:   pageToken,
			ScopeFilter: scopeFilter,
		}.Build())
	}
}

func cacheRangeKubeClustersWithScopeFilter(cache *Cache, scopeFilter *scopesv1.Filter) func(context.Context, string, string) iter.Seq2[types.KubeCluster, error] {
	return func(ctx context.Context, startKey, endKey string) iter.Seq2[types.KubeCluster, error] {
		return cache.RangeKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
			ScopeFilter: scopeFilter,
			PageToken:   startKey,
		}.Build())
	}
}

// TestKubernetesServers tests that CRUD operations on kube servers are
// replicated from the backend to the cache.
func TestKubernetesServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetKubernetesServers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeServer]{
			newResource: func(name string) (types.KubeServer, error) {
				app, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
				require.NoError(t, err)
				return types.NewKubernetesServerV3FromCluster(app, "host", uuid.New().String())
			},
			create:    withKeepalive(p.presenceS.UpsertKubernetesServer),
			list:      getAllAdapter(p.presenceS.GetKubernetesServers),
			cacheList: getAllAdapter(p.cache.GetKubernetesServers),
			update:    withKeepalive(p.presenceS.UpsertKubernetesServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllKubernetesServers(ctx)
			},
		}, withSkipPaginationTest())
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeServer]{
			newResource: func(name string) (types.KubeServer, error) {
				app, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
				require.NoError(t, err)
				return types.NewKubernetesServerV3FromCluster(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertKubernetesServer),
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeServer, string, error) {
				resources, next, err := listResource(ctx, p.presenceS, types.KindKubeServer, pageSize, pageToken)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				out, err := types.ResourcesWithLabels(resources).AsKubeServers()
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return out, next, nil
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeServer, string, error) {
				resources, next, err := listResource(ctx, p.cache, types.KindKubeServer, pageSize, pageToken)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				out, err := types.ResourcesWithLabels(resources).AsKubeServers()
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return out, next, nil
			},
			update: withKeepalive(p.presenceS.UpsertKubernetesServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllKubernetesServers(ctx)
			},
		})
	})
}

// TestListResourcesKubeServerScopedPagination verifies that ListResources
// pagination spanning unscoped and scoped kube servers returns every server
func TestListResourcesKubeServerScopedPagination(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		neverOK bool
	}{
		{name: "HealthyCache", neverOK: false},
		{name: "Fallback", neverOK: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				p := newTestPack(t, func(cfg Config) Config {
					cfg = ForAuth(cfg)
					cfg.neverOK = tt.neverOK
					return cfg
				})
				t.Cleanup(p.Close)

				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case <-p.eventsC:
							// Discard events to avoid blocking the test.
						}
					}
				}()

				key := func(s types.KubeServer) string {
					return s.GetScope() + "/" + s.GetHostID() + "/" + s.GetName()
				}

				var expectedKeys []string
				for _, scope := range []string{"", "", "", "/prod", "/prod", "/staging"} {
					server := mustCreateKubeServer(t, uuid.New().String(), "testcluster").(*types.KubernetesServerV3)
					server.Scope = scope
					server.Spec.Cluster.Scope = scope
					_, err := p.presenceS.UpsertKubernetesServer(ctx, server)
					require.NoError(t, err)
					expectedKeys = append(expectedKeys, key(server))
				}

				// Wait for the cache to replicate all servers.
				synctest.Wait()

				var actualKeys []string
				startKey := ""
				for {
					resp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
						ResourceType: types.KindKubeServer,
						Namespace:    apidefaults.Namespace,
						Limit:        1,
						StartKey:     startKey,
					})
					require.NoError(t, err)
					r := resp.Resources[0]
					server, ok := r.(types.KubeServer)
					require.True(t, ok)
					if startKey != "" { // first entry
						if startKey == scopes.ResourceCursorScopedStart() {
							// The unscoped range ended exactly at a page boundary;
							// next entry should be scoped
							require.NotEmpty(t, server.GetScope())
						} else {
							require.Equal(t, startKey, services.GetCursorForKubeServer(server))
						}
					}
					actualKeys = append(actualKeys, key(server))

					if resp.NextKey == "" {
						break
					}
					startKey = resp.NextKey
				}

				require.ElementsMatch(t, expectedKeys, actualKeys)
			})
		})
	}
}

func mustCreateKubeServer(t testing.TB, hostID, clusterName string) types.KubeServer {
	t.Helper()

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: clusterName,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3FromCluster(cluster, "localhost", hostID)
	require.NoError(t, err)
	return kubeServer
}

var kubeServerRangeFuncs = rangeServersWithTargetNameFuncs[types.KubeServer]{
	newResource: mustCreateKubeServer,
	create: func(ctx context.Context, presence services.Presence, s types.KubeServer) error {
		_, err := presence.UpsertKubernetesServer(ctx, s)
		return err
	},
	delete: func(ctx context.Context, presence services.Presence, s types.KubeServer) error {
		return presence.DeleteKubeServer(ctx, presencev1.DeleteKubeServerRequest_builder{
			HostId: s.GetHostID(),
			Name:   s.GetName(),
			Scope:  s.GetScope(),
		}.Build())
	},
	rangeByName: (*Cache).RangeKubernetesServersWithName,
}

func TestRangeKubernetesServersWithName(t *testing.T) {
	t.Parallel()
	testRangeServersWithTargetName(t, kubeServerRangeFuncs)
}

func BenchmarkRangeKubernetesServersWithName(b *testing.B) {
	benchmarkRangeServersWithTargetName(b, kubeServerRangeFuncs)
}

func TestKubernetesWaitingContainers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*kubewaitingcontainerpb.KubernetesWaitingContainer]{
		newResource: func(name string) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
			waitingCont, err := kubewaitingcontainer.NewKubeWaitingContainer(
				name,
				kubewaitingcontainerpb.KubernetesWaitingContainerSpec_builder{
					Username:      "user",
					Cluster:       "cluster",
					Namespace:     "namespace",
					PodName:       "pod",
					ContainerName: name,
					Patch:         []byte("{}"),
					PatchType:     "application/json-patch+json",
				}.Build())

			return waitingCont, trace.Wrap(err)
		},
		create: func(ctx context.Context, kwc *kubewaitingcontainerpb.KubernetesWaitingContainer) error {
			_, err := p.kubeWaitingContainers.CreateKubernetesWaitingContainer(ctx, kwc)
			return trace.Wrap(err)
		},
		cacheGet: func(ctx context.Context, name string) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
			return p.cache.GetKubernetesWaitingContainer(ctx, kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest_builder{
				Username:      "user",
				Cluster:       "cluster",
				Namespace:     "namespace",
				PodName:       "pod",
				ContainerName: name,
			}.Build())
		},
		list: func(ctx context.Context, i int, s string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error) {
			return p.kubeWaitingContainers.ListKubernetesWaitingContainers(ctx, i, s)
		},
		cacheList: func(ctx context.Context, i int, s string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error) {
			return p.cache.ListKubernetesWaitingContainers(ctx, i, s)
		},
		delete: func(ctx context.Context, s string) error {
			return p.kubeWaitingContainers.DeleteKubernetesWaitingContainer(ctx, kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest_builder{
				Username:      "user",
				Cluster:       "cluster",
				Namespace:     "namespace",
				PodName:       "pod",
				ContainerName: s,
			}.Build())
		},
		deleteAll: func(ctx context.Context) error {
			return p.kubeWaitingContainers.DeleteAllKubernetesWaitingContainers(ctx)
		},
	})

}
