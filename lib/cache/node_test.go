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
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
)

func newNodeResource(name string) (types.Server, error) {
	return NewServer(types.KindNode, name, "127.0.0.1:2022", apidefaults.Namespace), nil
}

// listNodesAdapter adapts a ListNodes method to the page function shape the
// resource test harness expects.
func listNodesAdapter(fn func(context.Context, *presencev1.ListSSHServersRequest) ([]types.Server, string, error)) func(context.Context, int, string) ([]types.Server, string, error) {
	return func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
		return fn(ctx, presencev1.ListSSHServersRequest_builder{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
		}.Build())
	}
}

// rangeNodesAdapter adapts a RangeSSHServers method to the range function shape the
// resource test harness expects.
func rangeNodesAdapter(fn func(context.Context, *presencev1.ListSSHServersRequest) iter.Seq2[types.Server, error]) func(context.Context, string, string) iter.Seq2[types.Server, error] {
	return func(ctx context.Context, start, _ string) iter.Seq2[types.Server, error] {
		return fn(ctx, presencev1.ListSSHServersRequest_builder{PageToken: start}.Build())
	}
}

// TestNodes tests nodes cache
func TestNodes(t *testing.T) {
	t.Parallel()

	t.Run("GetNodes", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: newNodeResource,
			create:      withKeepalive(p.presenceS.UpsertNode),
			list: getAllAdapter(func(ctx context.Context) ([]types.Server, error) {
				return p.presenceS.GetNodes(ctx, apidefaults.Namespace)
			}),
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetNode(ctx, apidefaults.Namespace, name)
			},
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.Server, error) { return p.cache.GetNodes(ctx, apidefaults.Namespace) }),
			update:    withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		}, withSkipPaginationTest())
	})

	t.Run("ListResources", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: newNodeResource,
			create:      withKeepalive(p.presenceS.UpsertNode),
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindNode,
					Limit:        int32(pageSize),
					StartKey:     pageToken,
				}

				var out []types.Server
				resp, err := p.presenceS.ListResources(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, s := range resp.Resources {
					out = append(out, s.(types.Server))
				}

				return out, resp.NextKey, nil
			},
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetNode(ctx, apidefaults.Namespace, name)
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindNode,
					Limit:        int32(pageSize),
					StartKey:     pageToken,
				}

				var out []types.Server
				resp, err := p.cache.ListResources(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, s := range resp.Resources {
					out = append(out, s.(types.Server))
				}

				return out, resp.NextKey, nil
			},
			update: withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		})
	})

	// GetSSHServer replaces GetNode as the single-node getter.
	t.Run("GetSSHServer", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: newNodeResource,
			create:      withKeepalive(p.presenceS.UpsertNode),
			list: getAllAdapter(func(ctx context.Context) ([]types.Server, error) {
				return p.presenceS.GetNodes(ctx, apidefaults.Namespace)
			}),
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{Name: name}.Build())
			},
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.Server, error) {
				return p.cache.GetNodes(ctx, apidefaults.Namespace)
			}),
			update: withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		}, withSkipPaginationTest())
	})

	t.Run("ListNodes", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: newNodeResource,
			create:      withKeepalive(p.presenceS.UpsertNode),
			list:        listNodesAdapter(p.presenceS.ListSSHServers),
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{Name: name}.Build())
			},
			cacheList: listNodesAdapter(p.cache.ListSSHServers),
			update:    withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		})
	})

	t.Run("RangeSSHServers", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy, ignoreRangeEndKey())
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: newNodeResource,
			create:      withKeepalive(p.presenceS.UpsertNode),
			list:        listNodesAdapter(p.presenceS.ListSSHServers),
			Range:       rangeNodesAdapter(p.presenceS.RangeSSHServers),
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{Name: name}.Build())
			},
			cacheList:  listNodesAdapter(p.cache.ListSSHServers),
			cacheRange: rangeNodesAdapter(p.cache.RangeSSHServers),
			update:     withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		})
	})
}

func BenchmarkGetMaxNodes(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping heavy benchmark")
	}
	benchGetNodes(b, 1_000_000)
}

func benchGetNodes(b *testing.B, nodeCount int) {
	p, err := newPack(b, ForAuth)
	require.NoError(b, err)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	createErr := make(chan error, 1)

	go func() {
		for range nodeCount {
			server := NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
			_, err := p.presenceS.UpsertNode(ctx, server)
			if err != nil {
				createErr <- err
				return
			}
		}
	}()

	timeout := time.After(time.Second * 90)

	for i := range nodeCount {
		select {
		case event := <-p.eventsC:
			if event.Type == RelativeExpiry {
				continue
			}

			require.Equal(b, EventProcessed, event.Type)
		case err := <-createErr:
			b.Fatalf("failed to create node: %v", err)
		case <-timeout:
			b.Fatalf("timeout waiting for event, progress=%d", i)
		}
	}

	b.ResetTimer()

	b.Run("GetNodes", func(b *testing.B) {
		for b.Loop() {
			nodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(b, err)
			require.Len(b, nodes, nodeCount)
		}
	})

	b.Run("ListResources", func(b *testing.B) {
		for b.Loop() {
			req := proto.ListResourcesRequest{
				ResourceType: types.KindNode,
			}

			nodes := make([]types.ResourceWithLabels, 0, nodeCount)
			for {
				resp, err := p.cache.ListResources(ctx, req)
				require.NoError(b, err)

				req.StartKey = resp.NextKey
				nodes = append(nodes, resp.Resources...)

				if req.StartKey == "" {
					break
				}
			}

			require.Len(b, nodes, nodeCount)
		}
	})

}

// TestNodeCollectionSeedHonorsWatchScopeFilter verifies the collection seed
// selects the same set of nodes as the event stream.
func TestNodeCollectionSeedHonorsWatchScopeFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	// Fixture names say which scope they live in.
	for name, scope := range map[string]string{
		"unscoped": "",
		"foo":      "/foo",
		"foobar":   "/foo/bar",
		"baz":      "/baz",
	} {
		node, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{}, nil)
		require.NoError(t, err)
		server, ok := node.(*types.ServerV2)
		require.True(t, ok, "expected *types.ServerV2, got %T", node)
		server.Scope = scope
		_, err = p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	for _, tc := range []struct {
		name        string
		scopeFilter *scopesv1.Filter
		want        []string
	}{
		{
			name:        "nil filter matches every scope",
			scopeFilter: nil,
			want:        []string{"foo", "foobar", "baz", "unscoped"},
		},
		{
			name:        "mode ALL matches every scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			want:        []string{"foo", "foobar", "baz", "unscoped"},
		},
		{
			name:        "mode UNSCOPED matches only unscoped",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			want:        []string{"unscoped"},
		},
		{
			name:        "mode EXACT matches one scope",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/foo"}.Build(),
			want:        []string{"foo"},
		},
		{
			name:        "mode DESCENDANTS matches the scope and below",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build(),
			want:        []string{"foo", "foobar"},
		},
		{
			name:        "mode ANCESTORS matches the scope and above",
			scopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: "/foo/bar"}.Build(),
			want:        []string{"foo", "foobar"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collection, err := newNodeCollection(p.presenceS, types.WatchKind{
				Kind:        types.KindNode,
				ScopeFilter: types.ScopeFilterFromProto(tc.scopeFilter),
			})
			require.NoError(t, err)

			seeded, err := collection.fetcher(ctx, false)
			require.NoError(t, err)

			var names []string
			for _, node := range seeded {
				names = append(names, node.GetName())
			}
			require.ElementsMatch(t, tc.want, names)
		})
	}
}
