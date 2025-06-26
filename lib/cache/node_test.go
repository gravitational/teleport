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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/suite"
)

// TestNodes tests nodes cache
func TestNodes(t *testing.T) {
	t.Parallel()

	t.Run("GetNodes", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: func(name string) (types.Server, error) {
				return suite.NewServer(types.KindNode, name, "127.0.0.1:2022", apidefaults.Namespace), nil
			},
			create: withKeepalive(p.presenceS.UpsertNode),
			list: func(ctx context.Context) ([]types.Server, error) {
				return p.presenceS.GetNodes(ctx, apidefaults.Namespace)
			},
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetNode(ctx, apidefaults.Namespace, name)
			},
			cacheList: func(ctx context.Context) ([]types.Server, error) {
				return p.cache.GetNodes(ctx, apidefaults.Namespace)
			},
			update: withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		})
	})

	t.Run("ListResources", func(t *testing.T) {
		t.Parallel()

		p := newTestPack(t, ForProxy)
		t.Cleanup(p.Close)

		testResources(t, p, testFuncs[types.Server]{
			newResource: func(name string) (types.Server, error) {
				return suite.NewServer(types.KindNode, name, "127.0.0.1:2022", apidefaults.Namespace), nil
			},
			create: withKeepalive(p.presenceS.UpsertNode),
			list: func(ctx context.Context) ([]types.Server, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindNode,
				}

				var out []types.Server
				for {
					resp, err := p.presenceS.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.Server))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			cacheGet: func(ctx context.Context, name string) (types.Server, error) {
				return p.cache.GetNode(ctx, apidefaults.Namespace, name)
			},
			cacheList: func(ctx context.Context) ([]types.Server, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindNode,
				}

				var out []types.Server
				for {
					resp, err := p.cache.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.Server))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			update: withKeepalive(p.presenceS.UpsertNode),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		})
	})
}

func BenchmarkGetMaxNodes(b *testing.B) {
	benchGetNodes(b, 1_000_000)
}

func benchGetNodes(b *testing.B, nodeCount int) {
	p, err := newPack(b.TempDir(), ForAuth, memoryBackend(true))
	require.NoError(b, err)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	createErr := make(chan error, 1)

	go func() {
		for range nodeCount {
			server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
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
